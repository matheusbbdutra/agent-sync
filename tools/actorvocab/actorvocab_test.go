package actorvocab

import (
	"strings"
	"testing"
)

func TestCLIsIncluiCline(t *testing.T) {
	if !IsCLI("cline") {
		t.Fatal("cline deveria estar na lista canônica de CLIs (6ª CLI, A-78)")
	}
	if got := len(CLIs()); got != 6 {
		t.Fatalf("esperava 6 CLIs canônicas, obteve %d: %v", got, CLIs())
	}
}

func TestActorsCobreCLIsEOrigemNaoCLI(t *testing.T) {
	actors := Actors()
	for _, want := range []string{"user", "tool", "agent-sync"} {
		if !contains(actors, want) {
			t.Errorf("actor %q ausente do enum: %v", want, actors)
		}
	}
	for _, cli := range CLIs() {
		if !contains(actors, cli) {
			t.Errorf("CLI %q ausente do enum de actors: %v", cli, actors)
		}
	}
}

// TestMemoryAgentsUsaVocabularioDoMCP guarda contra a divergência histórica:
// o memory-mcp anuncia "claude-code"/"antigravity", não "claude"/"agy".
func TestMemoryAgentsUsaVocabularioDoMCP(t *testing.T) {
	agents := MemoryAgents()
	for _, want := range []string{"claude-code", "antigravity", "cline", "user", "tool", "agent-sync"} {
		if !contains(agents, want) {
			t.Errorf("agent %q ausente do enum do memory-mcp: %v", want, agents)
		}
	}
	for _, wrong := range []string{"claude", "agy"} {
		if contains(agents, wrong) {
			t.Errorf("vocabulário vazou para o enum do MCP: %q em %v", wrong, agents)
		}
	}
}

func TestNormalizeActor(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"claude", "claude", true},
		{"cline", "cline", true},
		{"agent-sync", "agent-sync", true},
		{"user", "user", true},
		{"", "agent-sync", true},
		{"cli:meu-cli", "cli:meu-cli", true},
		{"cli:acme", "cli:acme", true},
		// Fechado por padrão: bare desconhecido não passa (precisa do namespace).
		{"meu-cli", "", false},
		{"aider", "", false},
		// Namespace malformado.
		{"cli:", "", false},
		{"cli:1x", "", false},
		{"cli:ACME", "", false},
		{"claude-code", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeActor(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("NormalizeActor(%q) = (%q, %v), esperado (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestNormalizeCLI(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"cline", "cline", true},
		{"claude", "claude", true},
		{"aider", "aider", true}, // CLI nova = slug, sem bump
		{"my-cli-2", "my-cli-2", true},
		{"", "", false},
		{"-bad", "", false},
		{"Bad", "", false},
		{"cli:cline", "", false}, // campo `cli` não usa namespace (o valor já é o CLI)
	}
	for _, tc := range cases {
		got, ok := NormalizeCLI(tc.in)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("NormalizeCLI(%q) = (%q, %v), esperado (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestMemoryAgent(t *testing.T) {
	cases := map[string]string{
		"claude":           "claude-code",
		"agy":              "antigravity",
		"cline":            "cline",
		"cli:claude":       "claude-code",
		"cli:agy":          "antigravity",
		"user":             "user",
		"tool":             "tool",
		"agent-sync":       "agent-sync",
		"":                 "agent-sync",
		"desconhecido":     "agent-sync",
		"cli:desconhecido": "desconhecido",
	}
	for in, want := range cases {
		if got := MemoryAgent(in); got != want {
			t.Errorf("MemoryAgent(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestPatternsAncorados(t *testing.T) {
	// O JSON Schema não ancora patterns por conta; se as constantes perderem o
	// ^/$ a validação passa a aceitar lixo com prefixo.
	for _, p := range []string{SlugPattern, NamespacedPattern} {
		if !strings.HasPrefix(p, "^") || !strings.HasSuffix(p, "$") {
			t.Errorf("pattern não ancorado: %q", p)
		}
	}
	if !strings.Contains(NamespacedPattern, Namespace+":") {
		t.Errorf("NamespacedPattern deveria conter o namespace: %q", NamespacedPattern)
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
