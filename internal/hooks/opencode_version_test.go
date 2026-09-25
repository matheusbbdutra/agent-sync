package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/opencode"
)

func TestParseOpencodeMajor(t *testing.T) {
	cases := map[string]int{
		"opencode v2.0.11":         2,
		"2.0.11":                   2,
		"opencode version 1.18.27": 1,
		"v1.0.0":                   1,
		"":                         0,
		"sem versao aqui":          0,
	}
	for in, want := range cases {
		if got := opencode.ParseMajor(in); got != want {
			t.Errorf("parseOpencodeMajor(%q)=%d want %d", in, got, want)
		}
	}
}

func TestOpencodeVersionOverride(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if got := opencode.MajorVersion(); got != 2 {
		t.Errorf("override 2: got %d", got)
	}
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	if got := opencode.MajorVersion(); got != 1 {
		t.Errorf("override 1: got %d", got)
	}
}

func TestOpencodePluginSuffix(t *testing.T) {
	if got := opencodePluginSuffix(1); got != ".opencode.ts" {
		t.Errorf("v1 suffix: got %q", got)
	}
	if got := opencodePluginSuffix(2); got != ".v2.ts" {
		t.Errorf("v2 suffix: got %q", got)
	}
}

// TestSyncOpenCodeV2PicksOpencodeV2Suffix valida que o wiring escolhe o
// padrão novo ".opencode.v2.ts" quando ambos existem (precedência do mais
// novo) e cai para ".v2.ts" quando só o legado existe.
func TestSyncOpenCodeV2PicksOpencodeV2Suffix(t *testing.T) {
	base := t.TempDir()
	hooks := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "precompact-snapshot.v2.ts"), []byte("// legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "precompact-snapshot.opencode.v2.ts"), []byte("// new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "token-nudge.opencode.v2.ts"), []byte("// new-only"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "ctx-window-nudge.opencode.v2.ts"), []byte("// a17-new"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}

	// Quando ambos existem, novo padrão ganha (.opencode.v2.ts).
	if err := syncOpenCodePrecompactSnapshotPlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "precompact-snapshot.ts"))
	if string(raw) != "// new" {
		t.Errorf("precompact-snapshot: novo padrão .opencode.v2.ts deve preceder, got %q", raw)
	}

	// Quando só o novo padrão existe, instala normalmente.
	if err := syncOpenCodeTokenNudgePlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(dest, "token-nudge.ts"))
	if string(raw) != "// new-only" {
		t.Errorf("token-nudge: deve instalar .opencode.v2.ts, got %q", raw)
	}

	// Plugin A-17 também funciona.
	if err := syncOpenCodeCtxWindowNudgePlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(dest, "ctx-window-nudge.ts"))
	if string(raw) != "// a17-new" {
		t.Errorf("ctx-window-nudge: deve instalar .opencode.v2.ts, got %q", raw)
	}
}

func TestSyncOpenCodeVersionedSelectsSource(t *testing.T) {
	base := t.TempDir()
	hooks := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "memory-nudge.opencode.ts"), []byte("// v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "memory-nudge.v2.ts"), []byte("// v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}
	if err := syncOpenCodeMemoryNudgePlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "memory-nudge.ts"))
	if string(raw) != "// v1" {
		t.Errorf("v1 deve instalar .opencode.ts, got %q", raw)
	}

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if err := syncOpenCodeMemoryNudgePlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(dest, "memory-nudge.ts"))
	if string(raw) != "// v2" {
		t.Errorf("v2 deve instalar .v2.ts, got %q", raw)
	}
}

func TestSyncOpenCodeNewPluginOnlyV2(t *testing.T) {
	base := t.TempDir()
	hooks := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "repo-map-warmup.v2.ts"), []byte("// v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	// Em v1, plugin novo é silencioso (sem erro, sem arquivo).
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}
	if err := syncOpenCodeRepoMapWarmupPlugin(base, tgt); err != nil {
		t.Fatalf("v1 novo plugin deve ser silencioso, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "repo-map-warmup.ts")); !os.IsNotExist(err) {
		t.Errorf("v1 não deve instalar plugin só-v2")
	}

	// Em v2 instala normalmente.
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if err := syncOpenCodeRepoMapWarmupPlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "repo-map-warmup.ts"))
	if string(raw) != "// v2" {
		t.Errorf("v2 deve instalar, got %q", raw)
	}
}
