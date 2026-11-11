package hooks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncImpeccableDesignAntigravity(t *testing.T) {
	base := t.TempDir()
	hooksDir := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{impeccableDesignScript, "wrap-hook.sh"} {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	settingsPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "antigravity",
		AgentKind:         "antigravity",
		HooksSettingsPath: settingsPath,
		HooksFormat:       "antigravity",
		HooksEvent:        "PreInvocation",
	}

	for i := 0; i < 2; i++ {
		if err := syncImpeccableDesignHook(base, target); err != nil {
			t.Fatalf("sync %d falhou: %v", i, err)
		}
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("json invalido: %s", data)
	}
	group, _ := root[impeccableDesignHookName].(map[string]any)
	if group == nil {
		t.Fatalf("hook %s ausente: %s", impeccableDesignHookName, data)
	}
	post, _ := group["PostToolUse"].([]any)
	pre, _ := group["PreInvocation"].([]any)
	if len(post) != 1 || len(pre) != 1 {
		t.Fatalf("esperava 1 entry em cada evento (idempotente), post=%d pre=%d: %s", len(post), len(pre), data)
	}
	if !strings.Contains(string(data), impeccableDesignScript) {
		t.Fatalf("comando sem o script do adapter: %s", data)
	}
	if !strings.Contains(string(data), "wrap-hook.sh") {
		t.Fatalf("comando sem wrap-hook.sh: %s", data)
	}
}

func TestSyncImpeccableDesignSkipsNonAntigravity(t *testing.T) {
	base := t.TempDir()
	settingsPath := filepath.Join(t.TempDir(), "hooks.json")
	target := TargetCLI{
		Name:              "opencode",
		AgentKind:         "opencode",
		HooksSettingsPath: settingsPath,
	}

	if err := syncImpeccableDesignHook(base, target); err != nil {
		t.Fatalf("no-op deveria ser silencioso: %v", err)
	}
	if _, err := os.Stat(settingsPath); err == nil {
		t.Fatal("hooks.json nao deveria ser criado para opencode")
	}
}

func TestImpeccableDesignAntigravityScript(t *testing.T) {
	fakeBin := filepath.Join(t.TempDir(), "agent-sync")
	if err := os.WriteFile(fakeBin, []byte("#!/usr/bin/env bash\ncat >/dev/null\nprintf '%s' '{\"context\":\"ok\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_BIN", fakeBin)
	t.Setenv("HOME", t.TempDir())

	script := filepath.Join("..", "..", "hooks", impeccableDesignScript)
	cmd := exec.Command("bash", script)
	cmd.Stdin = strings.NewReader(`{"conversationId":"c1"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("script falhou: %v", err)
	}
	if string(out) != `{"context":"ok"}` {
		t.Fatalf("stdout invalido (regressao do fallback ${out:-{}}): %q", out)
	}
}

func TestImpeccableDesignAntigravityScriptFailOpen(t *testing.T) {
	t.Setenv("AGENT_SYNC_BIN", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "/usr/bin:/bin")

	script := filepath.Join("..", "..", "hooks", impeccableDesignScript)
	cmd := exec.Command("bash", script)
	cmd.Stdin = strings.NewReader(`{}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("script falhou: %v", err)
	}
	if string(out) != "{}" {
		t.Fatalf("sem binario deveria ser fail-open {}: %q", out)
	}
}

func TestSyncOpenCodeImpeccableDesignPlugin(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	base := t.TempDir()
	hooksDir := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const marker = "// impeccable-design plugin"
	if err := os.WriteFile(filepath.Join(hooksDir, "impeccable-design.opencode.v2.ts"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginDir := t.TempDir()
	target := TargetCLI{
		Name:              "opencode",
		AgentKind:         "opencode",
		OpenCodePluginDir: pluginDir,
	}

	if err := syncOpenCodeImpeccableDesignPlugin(base, target); err != nil {
		t.Fatalf("sync falhou: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(pluginDir, "impeccable-design.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != marker {
		t.Fatalf("conteudo inesperado: %s", data)
	}
}
