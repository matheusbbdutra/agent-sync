package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/matheusdutra/token-tools/internal/secretscan"
)

// gitLabSubcommands permitidos. Read-only POR CONSTRUÇÃO: qualquer outro
// subcomando retorna erro antes de exec.Command. Adicionar um novo aqui é uma
// decisão consciente — diff/merge/approve/etc ficam fora por design.
var gitLabSubcommands = map[string]bool{
	"view": true, // glab mr view
	"diff": true, // glab mr diff
}

// validateGlabSubcommand retorna erro se sub não estiver na allowlist.
// Função pura extraída para teste sem precisar de exec.
func validateGlabSubcommand(sub string) error {
	if !gitLabSubcommands[sub] {
		return fmt.Errorf("subcomando glab %q não está na allowlist read-only", sub)
	}
	return nil
}

// remotePatterns reconhece as duas formas canônicas de URL GitLab:
//
//	git@gitlab.empresa.com:group/sub/proj.git
//	https://gitlab.empresa.com/group/sub/proj.git
//
// E extrai o host. Scheme/protocolo são descartados.
var (
	scpRemoteRE   = regexp.MustCompile(`^git@([^:]+):`)
	httpsRemoteRE = regexp.MustCompile(`^https?://([^/]+)/`)
)

// parseRemoteHost devolve o host inferido de uma URL remote Git ou string
// vazia + erro se não conseguir. Função pura (sem I/O) para facilitar testes.
func parseRemoteHost(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", errors.New("remote URL vazia")
	}
	if m := scpRemoteRE.FindStringSubmatch(remote); m != nil {
		return m[1], nil
	}
	if m := httpsRemoteRE.FindStringSubmatch(remote); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("formato de remote não reconhecido: %q", remote)
}

// gitRemoteURL obtém a URL do remote `origin` via exec de git. ctx é honrado.
func gitRemoteURL(ctx context.Context, repo string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin falhou: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runGlab executa `glab <subcommand>` validando contra a allowlist.
// O args após subcommand é responsabilidade do caller (e deve ser mínimo).
// Retorna (stdout, stderr, error). Erro de allowlist é reportado antes do exec.
func runGlab(ctx context.Context, host string, subcommand string, args ...string) (string, string, error) {
	if err := validateGlabSubcommand(subcommand); err != nil {
		return "", "", err
	}
	allArgs := []string{"mr", subcommand}
	if host != "" {
		allArgs = append(allArgs, "--hostname", host)
	}
	allArgs = append(allArgs, args...)
	cmd := exec.CommandContext(ctx, "glab", allArgs...)
	out, err := cmd.Output()
	// stderr fica disponível mesmo em sucesso para debug; em erro juntamos.
	stderr := ""
	if ee, ok := err.(*exec.ExitError); ok {
		stderr = strings.TrimSpace(string(ee.Stderr))
	}
	return strings.TrimSpace(string(out)), stderr, err
}

// glabMRView é o payload JSON de `glab mr view --output json`. Campos opcionais
// relevantes — só lemos o que precisamos para preencher reviewChange.
type glabMRView struct {
	SHA             string `json:"sha"`           // head SHA
	TargetBranch    string `json:"target_branch"` // base ref
	SourceBranch    string `json:"source_branch"` // head ref
	DiffRefsBaseSHA string `json:"diff_refs.base_sha"`
	DiffRefsHeadSHA string `json:"diff_refs.head_sha"`
	WebURL          string `json:"web_url"`
}

// collectGitLab é a entrada pública usada por main.go. Orquestra:
//  1. resolve host via git remote (ou usa -host se fornecido)
//  2. `glab mr view --output json` para metadados
//  3. `glab mr diff` para o patch
//  4. aplica redaction via secretscan
//  5. trunca diff se exceder maxBytes
func collectGitLab(ctx context.Context, in collectInput) (reviewChange, error) {
	host := in.Host
	if host == "" {
		remote, err := gitRemoteURL(ctx, in.Repo)
		if err != nil {
			return reviewChange{}, fmt.Errorf("resolver host do remote: %w", err)
		}
		h, err := parseRemoteHost(remote)
		if err != nil {
			return reviewChange{}, fmt.Errorf("parse de %q: %w", remote, err)
		}
		host = h
	}

	viewJSON, stderr, err := runGlab(ctx, host, "view", in.MRIID, "--output", "json")
	if err != nil {
		hint := ""
		if stderr != "" {
			hint = " — stderr glab: " + stderr
		}
		return reviewChange{}, fmt.Errorf("glab mr view falhou (host=%s): %w%s", host, err, hint)
	}
	var view glabMRView
	if err := json.Unmarshal([]byte(viewJSON), &view); err != nil {
		return reviewChange{}, fmt.Errorf("parse glab mr view JSON: %w", err)
	}

	diff, _, err := runGlab(ctx, host, "diff", in.MRIID)
	if err != nil {
		return reviewChange{}, fmt.Errorf("glab mr diff falhou (host=%s): %w", host, err)
	}

	// Redaction ANTES de truncar — evita cortar no meio de uma chave de API
	// sem flag de redacted.
	redactedDiff, found := secretscan.Redact(diff)
	truncated := false
	if len(redactedDiff) > in.MaxBytes {
		redactedDiff = redactedDiff[:in.MaxBytes]
		truncated = true
	}

	return reviewChange{
		Provider:       "gitlab",
		Repository:     view.WebURL,
		BaseRef:        view.TargetBranch,
		HeadRef:        view.SourceBranch,
		BaseSHA:        view.DiffRefsBaseSHA,
		HeadSHA:        view.DiffRefsHeadSHA,
		MergeBaseSHA:   "",         // glab mr diff não retorna merge-base de forma trivial; agente infere se precisar
		FetchedRemotes: []string{}, // exec glab não usa git fetch local
		Stat:           "",         // glab mr diff é texto puro, sem --stat confiável; deixamos vazio
		Diff:           redactedDiff,
		Truncated:      truncated,
		Redacted:       len(found) > 0,
	}, nil
}
