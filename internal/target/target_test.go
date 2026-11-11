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

	for _, expected := range []string{"claude", "codex", "antigravity", "opencode", "cline", "pi"} {
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

// TestPiTargetUsaAgentDir trava os paths do Pi: regras/skills/agents no agent
// dir e a extensão TS em extensions/ (Pi não tem hooks JSON). AgentsDir fica
// preenchido porque pi-sub-agent descobre ~/.pi/agent/agents/*.md.
func TestPiTargetUsaAgentDir(t *testing.T) {
	for _, trg := range GetTargetsForHome("/tmp/test-home") {
		if trg.Name != "pi" {
			continue
		}
		if trg.RulesPath != "/tmp/test-home/.pi/agent/AGENTS.md" {
			t.Errorf("RulesPath = %q", trg.RulesPath)
		}
		if trg.SkillsDir != "/tmp/test-home/.pi/agent/skills" {
			t.Errorf("SkillsDir = %q", trg.SkillsDir)
		}
		if trg.AgentsDir != "/tmp/test-home/.pi/agent/agents" {
			t.Errorf("AgentsDir = %q", trg.AgentsDir)
		}
		if trg.HookExtensionDir != "/tmp/test-home/.pi/agent/extensions" {
			t.Errorf("HookExtensionDir = %q", trg.HookExtensionDir)
		}
		if trg.HooksSettingsPath != "" {
			t.Errorf("HooksSettingsPath deveria ser vazio (hooks do Pi são extensões TS), obteve %q", trg.HooksSettingsPath)
		}
		if trg.HooksFormat != "pi" {
			t.Errorf("HooksFormat = %q, esperado pi", trg.HooksFormat)
		}
		return
	}
	t.Fatal("target pi não encontrado")
}
