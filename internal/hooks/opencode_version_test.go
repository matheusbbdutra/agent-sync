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
	// v1 aposentado: sufixo sempre v2 (hooks/*.opencode.ts deletados).
	if got := opencodePluginSuffix(1); got != ".v2.ts" {
		t.Errorf("v1 aposentado deve retornar sufixo v2, got %q", got)
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
	// v1 aposentado: arquivo .opencode.ts legado é ignorado; v2 instala .v2.ts.
	if err := os.WriteFile(filepath.Join(hooks, "memory-nudge.opencode.ts"), []byte("// v1-legado-ignorado"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "memory-nudge.v2.ts"), []byte("// v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	// Mesmo com override v1, instalador v2-only retorna erro explícito.
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}
	if err := syncOpenCodeMemoryNudgePlugin(base, tgt); err == nil {
		t.Errorf("v1 aposentado deve retornar erro explícito, got nil")
	}

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if err := syncOpenCodeMemoryNudgePlugin(base, tgt); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "memory-nudge.ts"))
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

	// v1 aposentado: erro explícito em vez de silencioso.
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}
	if err := syncOpenCodeRepoMapWarmupPlugin(base, tgt); err == nil {
		t.Errorf("v1 aposentado deve retornar erro explícito, got nil")
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

func TestSyncOpenCodeMemoryPruneCopiesScript(t *testing.T) {
	base := t.TempDir()
	hooks := filepath.Join(base, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "memory-prune-session-start.opencode.v2.ts"), []byte("// v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "memory-prune-session-start.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()

	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	tgt := TargetCLI{AgentKind: "opencode", OpenCodePluginDir: dest}
	if err := syncOpenCodeMemoryPruneSessionStartPlugin(base, tgt); err != nil {
		t.Fatal(err)
	}

	// O plugin resolve o bash por import.meta.dirname; o sync deve copiar ambos.
	ts, err := os.ReadFile(filepath.Join(dest, "memory-prune-session-start.ts"))
	if err != nil || string(ts) != "// v2" {
		t.Errorf("plugin TS nao copiado corretamente: %q, err=%v", ts, err)
	}
	sh, err := os.ReadFile(filepath.Join(dest, "memory-prune-session-start.sh"))
	if err != nil || string(sh) != "#!/usr/bin/env bash\n" {
		t.Errorf("script bash nao copiado corretamente: %q, err=%v", sh, err)
	}
}
