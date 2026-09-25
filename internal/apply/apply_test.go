package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/target"
)

// TestShouldDryRunReadsCachedOnly garante que shouldDryRun() não faz I/O
// depois de dryRunEnabled/dryRunEnvCache serem ajustados.
func TestShouldDryRunReadsCachedOnly(t *testing.T) {
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

	dryRunEnabled = false
	dryRunEnvCache = false
	t.Setenv("AGENT_SYNC_DRY_RUN", "1")
	if shouldDryRun() {
		t.Fatal("shouldDryRun leu env após cache ter sido desligado")
	}
}

// TestApplyDryRunDoesNotWriteDisk é o smoke test do dry-run: percorre o
// pipeline de applyToTarget em modo dry-run e garante que nenhum arquivo
// é criado em disco.
func TestApplyDryRunDoesNotWriteDisk(t *testing.T) {
	prevEnabled, prevEnv := dryRunEnabled, dryRunEnvCache
	t.Cleanup(func() { dryRunEnabled, dryRunEnvCache = prevEnabled, prevEnv })

	tempBase := t.TempDir()
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

	tempHome := t.TempDir()
	home, _ := os.UserHomeDir()
	t.Cleanup(func() { os.Setenv("HOME", home) })
	os.Setenv("HOME", tempHome)

	dryRunEnabled = true
	dryRunEnvCache = false

	tgt := target.TargetCLI{
		Name:              "claude",
		RulesPath:         filepath.Join(tempHome, ".claude", "AGENTS.md"),
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

	for _, p := range []string{tgt.RulesPath, tgt.SkillsDir, tgt.AgentsDir} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("dry-run criou arquivo indevidamente: %s", p)
		}
	}

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
