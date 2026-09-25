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

// githubSubcommands permitidos. Read-only POR CONSTRUÇÃO: qualquer outro
// subcomando retorna erro antes de exec.Command. Adicionar um novo aqui é uma
// decisão consciente — merge/close/review/etc ficam fora por design.
// Espelha exatamente o contrato de gitLabSubcommands.
var githubSubcommands = map[string]bool{
	"view": true, // gh pr view
	"diff": true, // gh pr diff
}

// validateGithubSubcommand retorna erro se sub não estiver na allowlist.
// Função pura extraída para teste sem precisar de exec.
func validateGithubSubcommand(sub string) error {
	if !githubSubcommands[sub] {
		return fmt.Errorf("subcomando gh %q não está na allowlist read-only", sub)
	}
	return nil
}

// ghRemoteRE reconhece as duas formas canônicas de URL GitHub:
//
//	git@github.com:owner/repo.git
//	https://github.com/owner/repo.git
//
// E extrai o host. Para self-hosted GitHub Enterprise (`git@gh.empresa.com:...`),
// a mesma regex cobre — basta o usuário definir GH_HOST antes de chamar.
// Esquema/protocolo são descartados.
var (
	ghSCPRemoteRE   = regexp.MustCompile(`^git@([^:]+):`)
	ghHTTPSRemoteRE = regexp.MustCompile(`^https?://([^/]+)/`)
)

// parseGithubRemoteHost devolve o host inferido de uma URL remote Git ou
// string vazia + erro se não conseguir. Função pura (sem I/O) para teste.
func parseGithubRemoteHost(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", errors.New("remote URL vazia")
	}
	if m := ghSCPRemoteRE.FindStringSubmatch(remote); m != nil {
		return m[1], nil
	}
	if m := ghHTTPSRemoteRE.FindStringSubmatch(remote); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("formato de remote não reconhecido: %q", remote)
}

// runGh executa `gh pr <subcommand>` validando contra a allowlist.
// O args após subcommand é responsabilidade do caller (e deve ser mínimo).
// Retorna (stdout, stderr, error). Erro de allowlist é reportado antes do exec.
//
// GitHub.com é o host padrão. Para self-hosted (GitHub Enterprise Server),
// o caller deve setar GH_HOST antes de chamar (runGh não força override).
func runGh(ctx context.Context, subcommand string, args ...string) (string, string, error) {
	if err := validateGithubSubcommand(subcommand); err != nil {
		return "", "", err
	}
	allArgs := []string{"pr", subcommand}
	allArgs = append(allArgs, args...)
	cmd := exec.CommandContext(ctx, "gh", allArgs...)
	out, err := cmd.Output()
	stderr := ""
	if ee, ok := err.(*exec.ExitError); ok {
		stderr = strings.TrimSpace(string(ee.Stderr))
	}
	return strings.TrimSpace(string(out)), stderr, err
}

// ghPRView é o payload JSON de `gh pr view <num> --json <fields>`. Campos
// opcionais relevantes — só lemos o que precisamos para preencher reviewChange.
// Documentação oficial: https://cli.github.com/manual/gh_pr_view (lida 2026-09-21).
type ghPRView struct {
	Number      int    `json:"number"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	BaseRefOid  string `json:"baseRefOid"`
	HeadRefOid  string `json:"headRefOid"`
	URL         string `json:"url"`
}

// collectGitHub é a entrada pública usada por main.go. Orquestra:
//  1. parse do remote para confirmar que estamos em um repo GitHub (sanidade)
//  2. `gh pr view <num> --json ...` para metadados
//  3. `gh pr diff <num> --color never` para o patch
//  4. aplica redaction via secretscan
//  5. trunca diff se exceder maxBytes
func collectGitHub(ctx context.Context, in collectInput) (reviewChange, error) {
	// Parse do remote só para SANIDADE (não usamos o host para forçar --hostname
	// porque gh não tem essa flag — usa GH_HOST env var). Se não conseguir
	// parsear, falhamos com mensagem clara pedindo GH_HOST ou remote válido.
	remote, err := gitRemoteURL(ctx, in.Repo)
	if err != nil {
		return reviewChange{}, fmt.Errorf("resolver host do remote: %w", err)
	}
	if _, perr := parseGithubRemoteHost(remote); perr != nil {
		return reviewChange{}, fmt.Errorf("repo %s não parece ser GitHub: %w (defina GH_HOST para self-hosted ou use -repo em checkout GitHub)", remote, perr)
	}

	viewJSON, stderr, err := runGh(ctx, "view", in.MRIID,
		"--json", "number,baseRefName,headRefName,baseRefOid,headRefOid,url")
	if err != nil {
		hint := ""
		if stderr != "" {
			hint = " — stderr gh: " + stderr
		}
		return reviewChange{}, fmt.Errorf("gh pr view falhou: %w%s", err, hint)
	}
	var view ghPRView
	if err := json.Unmarshal([]byte(viewJSON), &view); err != nil {
		return reviewChange{}, fmt.Errorf("parse gh pr view JSON: %w", err)
	}

	diff, _, err := runGh(ctx, "diff", in.MRIID, "--color", "never")
	if err != nil {
		return reviewChange{}, fmt.Errorf("gh pr diff falhou: %w", err)
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
		Provider:       "github",
		Repository:     view.URL,
		BaseRef:        view.BaseRefName,
		HeadRef:        view.HeadRefName,
		BaseSHA:        view.BaseRefOid,
		HeadSHA:        view.HeadRefOid,
		MergeBaseSHA:   "",         // gh pr diff não retorna merge-base; agente infere se precisar
		FetchedRemotes: []string{}, // exec gh não usa git fetch local
		Stat:           "",         // gh pr diff é texto puro, sem --stat confiável
		Diff:           redactedDiff,
		Truncated:      truncated,
		Redacted:       len(found) > 0,
	}, nil
}
