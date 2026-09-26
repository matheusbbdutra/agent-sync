package target

import (
	"testing"
)

func TestGetTargetsForHome(t *testing.T) {
	home := "/tmp/test-home"
	targets := GetTargetsForHome(home)
	if len(targets) != 6 {
		t.Fatalf("esperava 6 targets, obteve %d", len(targets))
	}

	names := map[string]bool{}
	for _, trg := range targets {
		names[trg.Name] = true
	}

	for _, expected := range []string{"claude", "codex", "antigravity", "opencode", "cursor", "cline"} {
		if !names[expected] {
			t.Errorf("target esperado ausente: %s", expected)
		}
	}
}

// TestClineNaoDeclaraAgentsDir trava a decisão do A-83: o Cline CLI não tem
// superfície de agents custom, então declarar AgentsDir traria de volta o
// warning "tipo de agente desconhecido" e uma entrega vazia. Se um dia o Cline
// suportar agents, atualize este teste junto com a evidência empírica.
func TestClineNaoDeclaraAgentsDir(t *testing.T) {
	for _, trg := range GetTargetsForHome("/tmp/test-home") {
		if trg.Name != "cline" {
			continue
		}
		if trg.AgentsDir != "" {
			t.Errorf("cline.AgentsDir deveria ser vazio (sem superfície de agents); obteve %q", trg.AgentsDir)
		}
		if trg.AgentKind != "cline" {
			t.Errorf("cline.AgentKind = %q, esperado cline (usado pelos hooks)", trg.AgentKind)
		}
		return
	}
	t.Fatal("target cline não encontrado")
}
