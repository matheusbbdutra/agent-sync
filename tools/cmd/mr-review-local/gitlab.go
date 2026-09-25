package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/secretscan"
)

// GitLabConfig é a configuração JSON carregada via -config.
// Apenas o NOME da variável de ambiente do token é persistido em disco;
// o valor nunca é gravado nem logado.
type GitLabConfig struct {
	BaseURL    string `json:"base_url"`
	Version    string `json:"version"`
	TokenEnv   string `json:"token_env"`
	TimeoutSec int    `json:"timeout_seconds"`
}

// validate normaliza defaults e verifica campos obrigatórios.
// version aceita "auto" (default) ou número major ("12", "13", ...).
func (c *GitLabConfig) validate() error {
	if c.BaseURL == "" {
		return errors.New("gitlab.base_url obrigatório")
	}
	if c.Version == "" {
		c.Version = "auto"
	}
	if c.Version != "auto" {
		if _, err := strconv.Atoi(c.Version); err != nil {
			return fmt.Errorf("gitlab.version inválida: %q", c.Version)
		}
	}
	if c.TokenEnv == "" {
		return errors.New("gitlab.token_env obrigatório")
	}
	if c.TimeoutSec <= 0 {
		c.TimeoutSec = 15
	}
	return nil
}

type gitLabProvider struct {
	baseURL  string
	version  string
	tokenEnv string
	client   *http.Client
}

func newGitLabProvider(cfg *GitLabConfig) (*gitLabProvider, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &gitLabProvider{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		version:  cfg.Version,
		tokenEnv: cfg.TokenEnv,
		client:   &http.Client{Timeout: time.Duration(cfg.TimeoutSec) * time.Second},
	}, nil
}

func (p *gitLabProvider) Name() string { return "gitlab" }

func (p *gitLabProvider) apiURL(path string) string {
	return p.baseURL + "/api/v4" + path
}

// token lê o valor do token da variável de ambiente nomeada em tokenEnv.
// O valor NUNCA é logado nem serializado.
func (p *gitLabProvider) token() (string, error) {
	tok := os.Getenv(p.tokenEnv)
	if tok == "" {
		return "", fmt.Errorf("variável de ambiente %q não definida ou vazia", p.tokenEnv)
	}
	return tok, nil
}

// doJSON executa GET e decodifica o body em out. Erros de auth são
// retornados como erro com status preservado.
func (p *gitLabProvider) doJSON(ctx context.Context, path string, out interface{}) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL(path), nil)
	if err != nil {
		return nil, err
	}
	tok, err := p.token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", tok)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp, fmt.Errorf("auth GitLab falhou: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	case resp.StatusCode >= 400:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp, fmt.Errorf("gitlab GET %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return resp, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp, fmt.Errorf("decode resposta %s: %w", path, err)
	}
	return resp, nil
}

// mrChangesResponse modela GET /merge_requests/:iid/changes (v4, todas versões).
// O campo overflow=true indica que o diff excedeu o limite do GitLab e foi
// omitido — sinaliza revisão parcial, não completa.
type mrChangesResponse struct {
	ID           int    `json:"id"`
	IID          int    `json:"iid"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	SHA          string `json:"sha"`
	DiffRefs     struct {
		BaseSHA  string `json:"base_sha"`
		HeadSHA  string `json:"head_sha"`
		StartSHA string `json:"start_sha"`
	} `json:"diff_refs"`
	Changes  []mrChange `json:"changes"`
	Overflow bool       `json:"overflow"`
}

type mrChange struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	NewFile     bool   `json:"new_file"`
	DeletedFile bool   `json:"deleted_file"`
	RenamedFile bool   `json:"renamed_file"`
	Diff        string `json:"diff"`
}

func (c mrChange) path() string {
	if c.NewPath != "" {
		return c.NewPath
	}
	return c.OldPath
}

// mrDiffResponse modela um item de GET /merge_requests/:iid/diffs (v13+).
type mrDiffResponse struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	AMode       string `json:"a_mode"`
	BMode       string `json:"b_mode"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	DeletedFile bool   `json:"deleted_file"`
	RenamedFile bool   `json:"renamed_file"`
}

func (d mrDiffResponse) path() string {
	if d.NewPath != "" {
		return d.NewPath
	}
	return d.OldPath
}

// useDiffsAPI decide se usa o endpoint /diffs (v13+) ou /changes (todas).
// Em "auto", o caller descobre via tentativa e fallback em runtime.
// Em versão explícita >= 13, usa /diffs sem tentativa.
func (p *gitLabProvider) useDiffsAPI() bool {
	if p.version == "auto" {
		return false // decidido em runtime via fallback
	}
	if major, err := strconv.Atoi(p.version); err == nil {
		return major >= 13
	}
	return false
}

// fetchChanges obtém /changes — sempre presente.
func (p *gitLabProvider) fetchChanges(ctx context.Context, projectPath, iid string) (*mrChangesResponse, error) {
	var out mrChangesResponse
	if _, err := p.doJSON(ctx, fmt.Sprintf("/projects/%s/merge_requests/%s/changes", projectPath, iid), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fetchDiffs pagina /diffs até X-Next-Page ausente. Em auto, retorna
// (nil, nil) se o endpoint não existir (404) para que o caller caia em /changes.
func (p *gitLabProvider) fetchDiffs(ctx context.Context, projectPath, iid string) ([]mrDiffResponse, bool, error) {
	const perPage = 100
	path := fmt.Sprintf("/projects/%s/merge_requests/%s/diffs?per_page=%d", projectPath, iid, perPage)
	page := 1
	var all []mrDiffResponse
	available := false
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL(path+"&page="+strconv.Itoa(page)), nil)
		if err != nil {
			return nil, false, err
		}
		tok, err := p.token()
		if err != nil {
			return nil, false, err
		}
		req.Header.Set("PRIVATE-TOKEN", tok)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, false, err
		}
		if page == 1 && resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return nil, false, nil // endpoint ausente: caller usa /changes
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			return nil, false, fmt.Errorf("gitlab GET /diffs: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var pageItems []mrDiffResponse
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			resp.Body.Close()
			return nil, false, fmt.Errorf("decode /diffs: %w", err)
		}
		resp.Body.Close()
		all = append(all, pageItems...)
		available = true
		next := resp.Header.Get("X-Next-Page")
		if next == "" {
			break
		}
		n, err := strconv.Atoi(next)
		if err != nil || n <= page {
			break
		}
		page = n
	}
	return all, available, nil
}

func (p *gitLabProvider) Fetch(ctx context.Context, in reviewInput, _ int) (reviewChange, error) {
	if in.Project == "" || in.MRIID == "" {
		return reviewChange{}, errors.New("gitlab: -project e -mr-iid obrigatórios")
	}
	projectPath := url.PathEscape(in.Project)
	mrIID := url.PathEscape(in.MRIID)

	changes, err := p.fetchChanges(ctx, projectPath, mrIID)
	if err != nil {
		return reviewChange{}, err
	}

	change := reviewChange{
		Provider:     "gitlab",
		Repository:   in.Project,
		BaseRef:      changes.TargetBranch,
		HeadRef:      changes.SourceBranch,
		BaseSHA:      changes.DiffRefs.BaseSHA,
		HeadSHA:      changes.DiffRefs.HeadSHA,
		MergeBaseSHA: changes.DiffRefs.StartSHA,
	}

	// Selecionar fonte: /diffs (v13+) ou /changes (todas).
	useDiffs := p.useDiffsAPI()
	if p.version == "auto" {
		diffs, available, derr := p.fetchDiffs(ctx, projectPath, mrIID)
		if derr != nil {
			return change, derr
		}
		if available && len(diffs) > 0 {
			useDiffs = true
		}
		if useDiffs {
			change.Stat, change.Diff = renderDiffs(diffs)
		}
	} else if useDiffs {
		diffs, available, derr := p.fetchDiffs(ctx, projectPath, mrIID)
		if derr != nil {
			return change, derr
		}
		if !available {
			return change, fmt.Errorf("gitlab %s não tem /diffs (versão %s)", p.baseURL, p.version)
		}
		change.Stat, change.Diff = renderDiffs(diffs)
	}

	if !useDiffs {
		change.Stat, change.Diff = renderChanges(changes.Changes)
	}

	change.Truncated = changes.Overflow
	if change.Truncated {
		change.Diff = ""
	}

	var found []string
	change.Stat, found = secretscan.Redact(change.Stat)
	change.Redacted = len(found) > 0
	change.Diff, found = secretscan.Redact(change.Diff)
	change.Redacted = change.Redacted || len(found) > 0

	if in.MaxBytes > 0 && len(change.Diff) > in.MaxBytes {
		change.Diff = ""
		change.Truncated = true
	}
	return change, nil
}

func renderDiffs(diffs []mrDiffResponse) (stat, diff string) {
	var sbStat, sbDiff strings.Builder
	for _, d := range diffs {
		path := d.path()
		if path == "" {
			continue
		}
		sbStat.WriteString(path)
		sbStat.WriteByte('\n')
		sbDiff.WriteString("diff --git a/" + path + " b/" + path + "\n")
		sbDiff.WriteString(d.Diff)
		if !strings.HasSuffix(d.Diff, "\n") {
			sbDiff.WriteByte('\n')
		}
	}
	return sbStat.String(), sbDiff.String()
}

func renderChanges(changes []mrChange) (stat, diff string) {
	var sbStat, sbDiff strings.Builder
	for _, c := range changes {
		path := c.path()
		if path == "" {
			continue
		}
		sbStat.WriteString(path)
		sbStat.WriteByte('\n')
		sbDiff.WriteString("diff --git a/" + path + " b/" + path + "\n")
		sbDiff.WriteString(c.Diff)
		if !strings.HasSuffix(c.Diff, "\n") {
			sbDiff.WriteByte('\n')
		}
	}
	return sbStat.String(), sbDiff.String()
}
