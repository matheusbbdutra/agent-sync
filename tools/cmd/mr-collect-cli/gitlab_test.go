package main

import (
	"strings"
	"testing"
)

func TestParseRemoteHost_SCP(t *testing.T) {
	cases := []struct {
		remote string
		want   string
		wantOK bool
	}{
		{"git@gitlab.com:group/proj.git", "gitlab.com", true},
		{"git@gitlab.empresa.com:group/sub/proj.git", "gitlab.empresa.com", true},
		{"git@gitlab.empresa.com:single/proj.git", "gitlab.empresa.com", true},
	}
	for _, tc := range cases {
		got, err := parseRemoteHost(tc.remote)
		if tc.wantOK && err != nil {
			t.Errorf("parseRemoteHost(%q) erro inesperado: %v", tc.remote, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseRemoteHost(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestParseRemoteHost_HTTPS(t *testing.T) {
	cases := []struct {
		remote string
		want   string
		wantOK bool
	}{
		{"https://gitlab.com/group/proj.git", "gitlab.com", true},
		{"https://gitlab.empresa.com/group/sub/proj.git", "gitlab.empresa.com", true},
		{"http://gitlab.empresa.com:8080/group/proj.git", "gitlab.empresa.com:8080", true},
	}
	for _, tc := range cases {
		got, err := parseRemoteHost(tc.remote)
		if tc.wantOK && err != nil {
			t.Errorf("parseRemoteHost(%q) erro inesperado: %v", tc.remote, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseRemoteHost(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestParseRemoteHost_Rejeitados(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"file:///tmp/repo.git",
		"not-a-url",
	}
	for _, tc := range cases {
		if _, err := parseRemoteHost(tc); err == nil {
			t.Errorf("parseRemoteHost(%q) devia falhar", tc)
		}
	}
}

// TestValidateGlabSubcommand_Allowlist garante read-only POR CONSTRUÇÃO:
// qualquer subcomando fora de {view, diff} falha antes do exec.Command.
// Este é o teste crítico de segurança: se alguém adicionar acidentalmente
// "merge" ou "approve" à allowlist, este teste pega.
func TestValidateGlabSubcommand_Allowlist(t *testing.T) {
	permitidos := []string{"view", "diff"}
	for _, s := range permitidos {
		if err := validateGlabSubcommand(s); err != nil {
			t.Errorf("validateGlabSubcommand(%q) devia permitir, err=%v", s, err)
		}
	}
	proibidos := []string{
		"merge", "approve", "close", "reopen", "delete", "subscribe",
		"create", "update", "edit", "note", "todo", "", "VIEW", // case-sensitive
	}
	for _, s := range proibidos {
		if err := validateGlabSubcommand(s); err == nil {
			t.Errorf("validateGlabSubcommand(%q) devia BLOQUEAR (read-only)", s)
		} else if !strings.Contains(err.Error(), "allowlist") {
			t.Errorf("validateGlabSubcommand(%q) erro sem menção a allowlist: %v", s, err)
		}
	}
}

// TestGitLabSubcommands_OnlyReadOnly é uma rede de proteção contra regressão
// silenciosa na allowlist. Se algum dia alguém adicionar um subcomando, este
// teste força a atualização da lista de "esperados".
func TestGitLabSubcommands_OnlyReadOnly(t *testing.T) {
	esperados := map[string]bool{"view": true, "diff": true}
	if len(gitLabSubcommands) != len(esperados) {
		t.Errorf("gitLabSubcommands mudou de tamanho: got %d, want %d", len(gitLabSubcommands), len(esperados))
	}
	for k := range gitLabSubcommands {
		if !esperados[k] {
			t.Errorf("subcomando %q na allowlist mas não nos esperados — atualize o teste intencionalmente", k)
		}
	}
}
