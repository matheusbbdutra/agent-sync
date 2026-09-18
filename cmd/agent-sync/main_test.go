package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSettingsPathDetailGating garante que settingsPathDetail suprime a
// linha de sucesso para targets sem HooksSettingsPath (ex.: opencode antes
// do plugin OpenCode ser escrito). Preserva o contrato das lambdas
// originais que foram fatoradas.
func TestSettingsPathDetailGating(t *testing.T) {
	if got := settingsPathDetail(TargetCLI{HooksSettingsPath: "/tmp/x/settings.json"}); got != "instalado em: /tmp/x/settings.json" {
		t.Errorf("settingsPathDetail com path: got %q", got)
	}
	if got := settingsPathDetail(TargetCLI{}); got != "" {
		t.Errorf("settingsPathDetail sem path: got %q want vazio", got)
	}
}

// TestOpenCodePluginDetailVariants cobre os dois formatos de mensagem
// para plugins OpenCode: com e sem a nota "ver README" (reflete o estado
// documentado na issue #13574).
func TestOpenCodePluginDetailVariants(t *testing.T) {
	tgt := TargetCLI{OpenCodePluginDir: "/home/u/.config/opencode/plugins"}
	if got := openCodePluginDetailNote(tgt); got != "em /home/u/.config/opencode/plugins (best-effort, ver README)" {
		t.Errorf("openCodePluginDetailNote: got %q", got)
	}
	if got := openCodePluginDetail(tgt); got != "em /home/u/.config/opencode/plugins (best-effort)" {
		t.Errorf("openCodePluginDetail: got %q", got)
	}
}

// TestStandardHooksHasExpectedEntries é um cinto-e-suspensórios sobre a
// tabela: garante que refactors futuros não removem acidentalmente
// entradas canônicas (o nome é o que aparece nos logs de progresso).
func TestStandardHooksHasExpectedEntries(t *testing.T) {
	want := map[string]bool{
		"context-guard":          false,
		"memory-nudge":           false,
		"agent-react":            false,
		"ctx-compact":            false,
		"ctx-handoff":            false,
		"shell-validate":         false,
		"docs-cache":             false,
		"stop":                   false,
		"preinvocation":          false,
		"opencode-context-guard": false,
		"opencode-memory":        false,
		"opencode-agent-react":   false,
		"opencode-ctx-compact":   false,
		"opencode-docs-cache":    false,
		"bash-guardian":          false,
	}
	for _, h := range standardHooks {
		if _, ok := want[h.name]; ok {
			want[h.name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("entrada obrigatória ausente em standardHooks: %q", name)
		}
	}
}

// TestShouldDryRunReadsCachedOnly garante que shouldDryRun() não faz I/O
// depois de dryRunEnabled/dryRunEnvCache serem ajustados. Sem o cache, o
// helper faria os.Getenv a cada chamada (em hooks rodando em loop).
func TestShouldDryRunReadsCachedOnly(t *testing.T) {
	// Salva e restaura estado global (outros testes podem ter alterado).
	prevEnabled, prevEnv := dryRunEnabled, dryRunEnvCache
	t.Cleanup(func() { dryRunEnabled, dryRunEnvCache = prevEnabled, prevEnv })

	t.Setenv("AGENT_SYNC_DRY_RUN", "")
	dryRunEnabled = false
	dryRunEnvCache = false
	if shouldDryRun() {
		t.Fatal("shouldDryRun=true sem flag nem env")
	}

	dryRunEnabled = true
	dryRunEnvCache = false
	if !shouldDryRun() {
		t.Fatal("shouldDryRun=false com flag habilitada")
	}

	dryRunEnabled = false
	dryRunEnvCache = true
	if !shouldDryRun() {
		t.Fatal("shouldDryRun=false com env cacheada")
	}

	// Mudar a env depois do cache NÃO deve afetar shouldDryRun — isso é o
	// que garante zero I/O no caminho quente.
	dryRunEnabled = false
	dryRunEnvCache = false
	t.Setenv("AGENT_SYNC_DRY_RUN", "1")
	if shouldDryRun() {
		t.Fatal("shouldDryRun leu env após cache ter sido desligado")
	}
}

// TestApplyDryRunDoesNotWriteDisk é o smoke test do dry-run: percorre o
// pipeline de applyToTarget em modo dry-run e garante que nenhum arquivo
// é criado em disco. Cobertura de copyFile/syncSkills/etc. via uma
// fixture mínima.
func TestApplyDryRunDoesNotWriteDisk(t *testing.T) {
	prevEnabled, prevEnv := dryRunEnabled, dryRunEnvCache
	t.Cleanup(func() { dryRunEnabled, dryRunEnvCache = prevEnabled, prevEnv })

	tempBase := t.TempDir()
	// Cria fonte de regras e skills para o apply.
	rulesSrc := filepath.Join(tempBase, "rules", "global-rules.md")
	if err := os.MkdirAll(filepath.Dir(rulesSrc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulesSrc, []byte("# Regras\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillsSrc := filepath.Join(tempBase, "skills")
	if err := os.MkdirAll(filepath.Join(skillsSrc, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsSrc, "demo", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Target com paths apontando para tempDir — se dry-run falhar, estes
	// paths aparecerão no disco depois do teste.
	tempHome := t.TempDir()
	home, _ := os.UserHomeDir()
	t.Cleanup(func() { os.Setenv("HOME", home) })
	os.Setenv("HOME", tempHome)

	dryRunEnabled = true
	dryRunEnvCache = false

	tgt := TargetCLI{
		Name:              "claude",
		RulesPath:         filepath.Join(tempHome, ".claude", "CLAUDE.md"),
		SkillsDir:         filepath.Join(tempHome, ".claude", "skills"),
		AgentsDir:         filepath.Join(tempHome, ".claude", "agents"),
		AgentKind:         "claude",
		HooksSettingsPath: filepath.Join(tempHome, ".claude", "settings.json"),
	}

	wl := &workerLog{}
	ctx := applyContext{
		baseDir:      tempBase,
		rulesSource:  rulesSrc,
		skillsSource: skillsSrc,
		log:          wl,
	}
	applyToTarget(ctx, tgt)

	// Nenhum arquivo deve ter sido criado em tempHome.
	for _, p := range []string{tgt.RulesPath, tgt.SkillsDir, tgt.AgentsDir} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("dry-run criou arquivo indevidamente: %s", p)
		}
	}

	// E deve haver ao menos uma linha de log indicando o no-op.
	found := false
	for _, line := range wl.lines() {
		if strings.Contains(line, "[dry-run]") || strings.Contains(line, "Regras atualizadas") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("dry-run não emitiu nenhuma linha informativa; logs=%v", wl.lines())
	}
}
