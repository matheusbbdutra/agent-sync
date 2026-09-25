package hooks

import (
	"testing"
)

func TestSettingsPathDetailGating(t *testing.T) {
	if got := settingsPathDetail(TargetCLI{HooksSettingsPath: "/tmp/x/settings.json"}); got != "instalado em: /tmp/x/settings.json" {
		t.Errorf("settingsPathDetail com path: got %q", got)
	}
	if got := settingsPathDetail(TargetCLI{}); got != "" {
		t.Errorf("settingsPathDetail sem path: got %q want vazio", got)
	}
}

func TestOpenCodePluginDetailVariants(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	tgt := TargetCLI{OpenCodePluginDir: "/home/u/.config/opencode/plugins"}
	if got := openCodePluginDetailNote(tgt); got != "em /home/u/.config/opencode/plugins (best-effort, ver README)" {
		t.Errorf("openCodePluginDetailNote v1: got %q", got)
	}
	if got := openCodePluginDetail(tgt); got != "em /home/u/.config/opencode/plugins (best-effort)" {
		t.Errorf("openCodePluginDetail v1: got %q", got)
	}

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if got := openCodePluginDetailNote(tgt); got != "em /home/u/.config/opencode/plugins (v2, ver README)" {
		t.Errorf("openCodePluginDetailNote v2: got %q", got)
	}
	if got := openCodePluginDetail(tgt); got != "em /home/u/.config/opencode/plugins (v2)" {
		t.Errorf("openCodePluginDetail v2: got %q", got)
	}
}

func TestStandardHooksHasExpectedEntries(t *testing.T) {
	want := map[string]bool{
		"context-guard":            false,
		"memory-nudge":             false,
		"agent-react":              false,
		"ctx-compact":              false,
		"ctx-handoff":              false,
		"shell-validate":           false,
		"docs-cache":               false,
		"stop":                     false,
		"preinvocation":            false,
		"opencode-context-guard":   false,
		"opencode-memory":          false,
		"opencode-agent-react":     false,
		"opencode-ctx-compact":     false,
		"opencode-docs-cache":      false,
		"opencode-repo-map-warmup": false,
		"bash-guardian":            false,
	}
	for _, name := range StandardHookNames() {
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("entrada obrigatória ausente em standardHooks: %q", name)
		}
	}
}
