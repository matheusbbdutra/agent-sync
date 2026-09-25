package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// mockGitLab monta um servidor httptest que responde a /api/v4 conforme
// a config passada (versionMode = "v12", "v13" ou "auto").
// Retorna o baseURL e uma função para inspecionar requests recebidos.
type mockGitLab struct {
	server      *httptest.Server
	diffsHits   int
	changesHits int
}

func newMockGitLab(t *testing.T, mode string) *mockGitLab {
	t.Helper()
	m := &mockGitLab{}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v4/projects/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v4/projects/")
		// Rotas esperadas:
		//   /projects/:id/merge_requests/:iid/changes
		//   /projects/:id/merge_requests/:iid/diffs
		if strings.HasSuffix(path, "/diffs") {
			m.diffsHits++
			if mode == "v12" {
				http.NotFound(w, r)
				return
			}
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				w.Header().Set("X-Next-Page", "")
				_ = json.NewEncoder(w).Encode([]mrDiffResponse{
					{
						OldPath: "flow.go",
						NewPath: "flow.go",
						Diff:    "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n",
					},
					{
						NewPath: "new.go",
						NewFile: true,
						Diff:    "@@ -0,0 +1 @@\n+package new\n",
					},
				})
				return
			}
			// página > 1: vazia, fim da paginação
			w.Header().Set("X-Next-Page", "")
			_ = json.NewEncoder(w).Encode([]mrDiffResponse{})
			return
		}
		if strings.HasSuffix(path, "/changes") {
			m.changesHits++
			_ = json.NewEncoder(w).Encode(mrChangesResponse{
				ID:           1,
				IID:          1,
				SourceBranch: "feature",
				TargetBranch: "main",
				SHA:          "head-sha",
				DiffRefs: struct {
					BaseSHA  string `json:"base_sha"`
					HeadSHA  string `json:"head_sha"`
					StartSHA string `json:"start_sha"`
				}{
					BaseSHA:  "base-sha",
					HeadSHA:  "head-sha",
					StartSHA: "merge-base-sha",
				},
				Changes: []mrChange{
					{
						OldPath: "flow.go",
						NewPath: "flow.go",
						Diff:    "@@ -1 +1 @@\n-func Old() {}\n+func New() {}\n",
					},
				},
			})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	m.server = httptest.NewServer(mux)
	t.Cleanup(m.server.Close)
	return m
}

func TestGitLabConfigValidateDefaults(t *testing.T) {
	cfg := &GitLabConfig{BaseURL: "https://gitlab.example.com", TokenEnv: "GITLAB_TOKEN"}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate default: %v", err)
	}
	if cfg.Version != "auto" {
		t.Fatalf("version default: %q", cfg.Version)
	}
	if cfg.TimeoutSec != 15 {
		t.Fatalf("timeout default: %d", cfg.TimeoutSec)
	}
}

func TestGitLabConfigValidateRejectsInvalidVersion(t *testing.T) {
	cfg := &GitLabConfig{BaseURL: "https://x", Version: "banana", TokenEnv: "T"}
	if err := cfg.validate(); err == nil {
		t.Fatal("esperava erro para version não-numérica")
	}
}

func TestGitLabConfigValidateRequiresBaseURLAndTokenEnv(t *testing.T) {
	if err := (&GitLabConfig{TokenEnv: "T"}).validate(); err == nil {
		t.Fatal("esperava erro sem base_url")
	}
	if err := (&GitLabConfig{BaseURL: "https://x"}).validate(); err == nil {
		t.Fatal("esperava erro sem token_env")
	}
}

func TestDispatchProviderRejectsUnknown(t *testing.T) {
	if _, err := dispatchProvider(reviewInput{Provider: "bitbucket"}); err == nil {
		t.Fatal("esperava erro para provider desconhecido")
	}
}

func TestDispatchProviderGitLabRequiresConfig(t *testing.T) {
	if _, err := dispatchProvider(reviewInput{Provider: "gitlab"}); err == nil {
		t.Fatal("esperava erro sem GitLabConfig")
	}
}

func TestDispatchProviderLocalDefaults(t *testing.T) {
	p, err := dispatchProvider(reviewInput{})
	if err != nil {
		t.Fatalf("dispatch local: %v", err)
	}
	if p.Name() != "local" {
		t.Fatalf("nome: %q", p.Name())
	}
}

func TestGitLabProviderFetchChangesV12(t *testing.T) {
	mock := newMockGitLab(t, "v12")
	t.Setenv("GITLAB_TOKEN", "test-token")
	prov, err := newGitLabProvider(&GitLabConfig{
		BaseURL:    mock.server.URL,
		Version:    "12",
		TokenEnv:   "GITLAB_TOKEN",
		TimeoutSec: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	change, err := prov.Fetch(context.Background(), reviewInput{
		Provider: "gitlab",
		Project:  "group/project",
		MRIID:    "42",
		MaxBytes: 4096,
	}, 4096)
	if err != nil {
		t.Fatalf("Fetch v12: %v", err)
	}
	if change.Provider != "gitlab" {
		t.Fatalf("provider: %q", change.Provider)
	}
	if change.BaseRef != "main" || change.HeadRef != "feature" {
		t.Fatalf("refs: %+v", change)
	}
	if change.BaseSHA != "base-sha" || change.HeadSHA != "head-sha" || change.MergeBaseSHA != "merge-base-sha" {
		t.Fatalf("SHAs: %+v", change)
	}
	if mock.diffsHits != 0 {
		t.Fatalf("v12 não deveria hit /diffs, mas hit %d", mock.diffsHits)
	}
	if mock.changesHits == 0 {
		t.Fatal("/changes não foi chamado")
	}
	if !strings.Contains(change.Diff, "func New()") {
		t.Fatalf("diff não contém mudança esperada: %s", change.Diff)
	}
}

func TestGitLabProviderFetchDiffsV13(t *testing.T) {
	mock := newMockGitLab(t, "v13")
	t.Setenv("GITLAB_TOKEN", "test-token")
	prov, err := newGitLabProvider(&GitLabConfig{
		BaseURL:    mock.server.URL,
		Version:    "13",
		TokenEnv:   "GITLAB_TOKEN",
		TimeoutSec: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	change, err := prov.Fetch(context.Background(), reviewInput{
		Provider: "gitlab",
		Project:  "group/project",
		MRIID:    "42",
		MaxBytes: 4096,
	}, 4096)
	if err != nil {
		t.Fatalf("Fetch v13: %v", err)
	}
	if mock.diffsHits == 0 {
		t.Fatal("/diffs não foi chamado")
	}
	if !strings.Contains(change.Diff, "new.go") {
		t.Fatalf("/diffs devia trazer new.go (arquivo novo): %s", change.Diff)
	}
}

func TestGitLabProviderFetchAutoFallbackToChanges(t *testing.T) {
	mock := newMockGitLab(t, "v12") // /diffs retorna 404
	t.Setenv("GITLAB_TOKEN", "test-token")
	prov, err := newGitLabProvider(&GitLabConfig{
		BaseURL:  mock.server.URL,
		Version:  "auto",
		TokenEnv: "GITLAB_TOKEN",
	})
	if err != nil {
		t.Fatal(err)
	}
	change, err := prov.Fetch(context.Background(), reviewInput{
		Provider: "gitlab",
		Project:  "group/project",
		MRIID:    "42",
		MaxBytes: 4096,
	}, 4096)
	if err != nil {
		t.Fatalf("Fetch auto: %v", err)
	}
	if mock.diffsHits == 0 {
		t.Fatal("auto deveria tentar /diffs antes do fallback")
	}
	if mock.changesHits == 0 {
		t.Fatal("auto deveria cair em /changes após 404")
	}
	if !strings.Contains(change.Diff, "func New()") {
		t.Fatalf("diff esperado: %s", change.Diff)
	}
}

func TestGitLabProviderFetchRequiresProjectAndIID(t *testing.T) {
	mock := newMockGitLab(t, "v13")
	t.Setenv("GITLAB_TOKEN", "test-token")
	prov, err := newGitLabProvider(&GitLabConfig{BaseURL: mock.server.URL, Version: "13", TokenEnv: "GITLAB_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prov.Fetch(context.Background(), reviewInput{}, 4096); err == nil {
		t.Fatal("esperava erro sem project/mr-iid")
	}
}

func TestGitLabProviderFetchAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "token invalid", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "wrong")
	prov, _ := newGitLabProvider(&GitLabConfig{BaseURL: server.URL, Version: "13", TokenEnv: "GITLAB_TOKEN"})
	_, err := prov.Fetch(context.Background(), reviewInput{Provider: "gitlab", Project: "p", MRIID: "1", MaxBytes: 1024}, 1024)
	if err == nil || !strings.Contains(err.Error(), "auth GitLab falhou") {
		t.Fatalf("esperava erro de auth, got %v", err)
	}
}

func TestGitLabProviderFetchMissingTokenEnv(t *testing.T) {
	mock := newMockGitLab(t, "v13")
	os.Unsetenv("GITLAB_TOKEN_NEVER_SET")
	prov, _ := newGitLabProvider(&GitLabConfig{BaseURL: mock.server.URL, Version: "13", TokenEnv: "GITLAB_TOKEN_NEVER_SET"})
	_, err := prov.Fetch(context.Background(), reviewInput{Provider: "gitlab", Project: "p", MRIID: "1", MaxBytes: 1024}, 1024)
	if err == nil || !strings.Contains(err.Error(), "GITLAB_TOKEN_NEVER_SET") {
		t.Fatalf("esperava erro de token_env ausente, got %v", err)
	}
}

func TestGitLabProviderFetchOverflowMarksTruncated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/changes") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":            1,
				"iid":           1,
				"source_branch": "feat",
				"target_branch": "main",
				"sha":           "h",
				"diff_refs":     map[string]string{"base_sha": "b", "head_sha": "h", "start_sha": "m"},
				"changes":       []map[string]string{{"old_path": "a.go", "new_path": "a.go", "diff": "@@ -1 +1 @@\n-x\n+y\n"}},
				"overflow":      true,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "t")
	prov, _ := newGitLabProvider(&GitLabConfig{BaseURL: server.URL, Version: "12", TokenEnv: "GITLAB_TOKEN"})
	change, err := prov.Fetch(context.Background(), reviewInput{Provider: "gitlab", Project: "p", MRIID: "1", MaxBytes: 4096}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !change.Truncated || change.Diff != "" {
		t.Fatalf("overflow deveria truncar e limpar diff: %+v", change)
	}
}

func TestGitLabProviderFetchRedactsSecret(t *testing.T) {
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/changes") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "iid": 1,
				"source_branch": "feat", "target_branch": "main", "sha": "h",
				"diff_refs": map[string]string{"base_sha": "b", "head_sha": "h", "start_sha": "m"},
				"changes": []map[string]string{
					{"old_path": "secret.txt", "new_path": "secret.txt", "diff": fmt.Sprintf("@@ -0,0 +1 @@\n+%s\n", secret)},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	t.Setenv("GITLAB_TOKEN", "t")
	prov, _ := newGitLabProvider(&GitLabConfig{BaseURL: server.URL, Version: "12", TokenEnv: "GITLAB_TOKEN"})
	change, err := prov.Fetch(context.Background(), reviewInput{Provider: "gitlab", Project: "p", MRIID: "1", MaxBytes: 4096}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if !change.Redacted {
		t.Fatal("redaction não disparou")
	}
	if strings.Contains(change.Diff, secret) {
		t.Fatalf("segredo vazou no diff: %s", change.Diff)
	}
}

func TestGitLabProviderFetchRespectsMaxBytes(t *testing.T) {
	mock := newMockGitLab(t, "v13")
	t.Setenv("GITLAB_TOKEN", "t")
	prov, _ := newGitLabProvider(&GitLabConfig{BaseURL: mock.server.URL, Version: "13", TokenEnv: "GITLAB_TOKEN"})
	change, err := prov.Fetch(context.Background(), reviewInput{
		Provider: "gitlab", Project: "p", MRIID: "1", MaxBytes: 10,
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !change.Truncated || change.Diff != "" {
		t.Fatalf("max-bytes deveria truncar: %+v", change)
	}
}
