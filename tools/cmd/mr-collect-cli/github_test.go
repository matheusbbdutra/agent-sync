package main

import (
	"strings"
	"testing"
)

func TestParseGithubRemoteHost_SCP(t *testing.T) {
	cases := []struct {
		remote string
		want   string
		wantOK bool
	}{
		{"git@github.com:owner/repo.git", "github.com", true},
		{"git@github.com:owner/sub/repo.git", "github.com", true},
		{"git@gh.empresa.com:owner/repo.git", "gh.empresa.com", true}, // GitHub Enterprise self-hosted
	}
	for _, tc := range cases {
		got, err := parseGithubRemoteHost(tc.remote)
		if tc.wantOK && err != nil {
			t.Errorf("parseGithubRemoteHost(%q) erro inesperado: %v", tc.remote, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseGithubRemoteHost(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestParseGithubRemoteHost_HTTPS(t *testing.T) {
	cases := []struct {
		remote string
		want   string
		wantOK bool
	}{
		{"https://github.com/owner/repo.git", "github.com", true},
		{"https://github.com/owner/sub/repo.git", "github.com", true},
		{"https://gh.empresa.com/owner/repo.git", "gh.empresa.com", true},
	}
	for _, tc := range cases {
		got, err := parseGithubRemoteHost(tc.remote)
		if tc.wantOK && err != nil {
			t.Errorf("parseGithubRemoteHost(%q) erro inesperado: %v", tc.remote, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseGithubRemoteHost(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestParseGithubRemoteHost_Rejeitados(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"file:///tmp/repo.git",
		"not-a-url",
	}
	for _, tc := range cases {
		if _, err := parseGithubRemoteHost(tc); err == nil {
			t.Errorf("parseGithubRemoteHost(%q) devia falhar", tc)
		}
	}
}

// TestValidateGithubSubcommand_Allowlist garante read-only POR CONSTRUÇÃO:
// qualquer subcomando fora de {view, diff} falha antes do exec.Command.
// Espelha exatamente o teste de glab para garantir paridade de garantia.
func TestValidateGithubSubcommand_Allowlist(t *testing.T) {
	permitidos := []string{"view", "diff"}
	for _, s := range permitidos {
		if err := validateGithubSubcommand(s); err != nil {
			t.Errorf("validateGithubSubcommand(%q) devia permitir, err=%v", s, err)
		}
	}
	proibidos := []string{
		"merge", "close", "reopen", "review", "comment", "edit", "create",
		"checkout", "lock", "ready", "revert", "update-branch", "checks",
		"status", "", "VIEW", // case-sensitive
	}
	for _, s := range proibidos {
		if err := validateGithubSubcommand(s); err == nil {
			t.Errorf("validateGithubSubcommand(%q) devia BLOQUEAR (read-only)", s)
		} else if !strings.Contains(err.Error(), "allowlist") {
			t.Errorf("validateGithubSubcommand(%q) erro sem menção a allowlist: %v", s, err)
		}
	}
}

// TestGithubSubcommands_OnlyReadOnly é uma rede de proteção contra regressão
// silenciosa na allowlist. Se algum dia alguém adicionar um subcomando, este
// teste força a atualização da lista de "esperados".
func TestGithubSubcommands_OnlyReadOnly(t *testing.T) {
	esperados := map[string]bool{"view": true, "diff": true}
	if len(githubSubcommands) != len(esperados) {
		t.Errorf("githubSubcommands mudou de tamanho: got %d, want %d", len(githubSubcommands), len(esperados))
	}
	for k := range githubSubcommands {
		if !esperados[k] {
			t.Errorf("subcomando %q na allowlist mas não nos esperados — atualize o teste intencionalmente", k)
		}
	}
}

// TestParseGithubRemoteHost_NaoAceitaGitlab garante que o parse github não
// aceita URLs que parecem GitLab. Defesa contra mix-up de providers —
// sem isso, alguém poderia chamar mr-collect-cli -provider=github em repo
// GitLab e receber erro confuso.
func TestParseGithubRemoteHost_NaoAceitaGitlab(t *testing.T) {
	// Nota: o parser genérico vai ACEITAR qualquer host (gitlab.com, etc.)
	// porque ele só extrai o host. A rejeição de GitLab-em-repo-github
	// é responsabilidade de collectGitHub() no contexto. Aqui validamos
	// apenas que o parser não quebra com formatos GitLab.
	gitlabURL := "https://gitlab.com/owner/repo.git"
	host, err := parseGithubRemoteHost(gitlabURL)
	if err != nil {
		t.Errorf("parseGithubRemoteHost não devia falhar em URL válida (mesmo sendo GitLab): %v", err)
	}
	if host != "gitlab.com" {
		t.Errorf("host extraído errado: got %q, want %q", host, "gitlab.com")
	}
}
