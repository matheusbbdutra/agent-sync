package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Teste", "GIT_AUTHOR_EMAIL=teste@example.invalid", "GIT_COMMITTER_NAME=Teste", "GIT_COMMITTER_EMAIL=teste@example.invalid")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, output)
	}
	return strings.TrimSpace(string(output))
}

func testRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "flow.go"), []byte("package flow\nfunc Value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "flow.go")
	runGit(t, repo, "commit", "-m", "base")
	runGit(t, repo, "branch", "feature")
	runGit(t, repo, "checkout", "feature")
	if err := os.WriteFile(filepath.Join(repo, "flow.go"), []byte("package flow\nfunc Value() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "commit", "-am", "mudança")
	return repo
}

func TestCollectLocalComparison(t *testing.T) {
	repo := testRepository(t)
	change, err := collect(context.Background(), reviewInput{Repo: repo, Base: "main", Head: "feature", MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if change.MergeBaseSHA != change.BaseSHA || change.HeadSHA == change.BaseSHA {
		t.Fatalf("SHAs incorretos: %+v", change)
	}
	if !strings.Contains(change.Diff, "+func Value() int { return 2 }") || change.Truncated {
		t.Fatalf("diff incorreto: %+v", change)
	}
	var output bytes.Buffer
	if err := run([]string{"-repo", repo, "-base", "main", "-head", "feature"}, &output); err != nil {
		t.Fatal(err)
	}
	var decoded reviewChange
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil || decoded.HeadSHA != change.HeadSHA {
		t.Fatalf("saída JSON inválida: %v %+v", err, decoded)
	}
}

func TestCollectRejectsInvalidRefAndReportsTruncation(t *testing.T) {
	repo := testRepository(t)
	if _, err := collect(context.Background(), reviewInput{Repo: repo, Base: "--bad", Head: "feature", MaxBytes: 100}); err == nil {
		t.Fatal("ref iniciada com hífen deveria ser recusada")
	}
	change, err := collect(context.Background(), reviewInput{Repo: repo, Base: "main", Head: "feature", MaxBytes: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !change.Truncated || change.Diff != "" {
		t.Fatal("patch limitado deveria ser omitido e sinalizar truncamento")
	}
}

func TestFetchRequiresRemote(t *testing.T) {
	repo := testRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := collect(ctx, reviewInput{Repo: repo, Base: "main", Head: "feature", Fetch: true, MaxBytes: 4096})
	if err == nil || !strings.Contains(err.Error(), "nenhum remoto") {
		t.Fatalf("esperava erro por falta de remoto: %v", err)
	}
}

func TestCollectFetchesNamedRemote(t *testing.T) {
	repo := testRepository(t)
	runGit(t, repo, "remote", "add", "origin", repo)
	change, err := collect(context.Background(), reviewInput{Repo: repo, Base: "origin/main", Head: "origin/feature", Fetch: true, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.FetchedRemotes) != 1 || change.FetchedRemotes[0] != "origin" || change.BaseSHA == change.HeadSHA {
		t.Fatalf("fetch/refs incorretos: %+v", change)
	}
}

func TestCollectRedactsKnownSecret(t *testing.T) {
	repo := testRepository(t)
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	if err := os.WriteFile(filepath.Join(repo, "secret.txt"), []byte(secret+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "secret.txt")
	runGit(t, repo, "commit", "-m", "amostra")
	change, err := collect(context.Background(), reviewInput{Repo: repo, Base: "main", Head: "feature", MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if !change.Redacted || strings.Contains(change.Diff, secret) || !strings.Contains(change.Diff, "[REDACTED:generic-api-key]") {
		t.Fatalf("redaction falhou: %+v", change)
	}
}
