package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeVersionAtLeast(t *testing.T) {
	cases := map[string]struct {
		ver                string
		minMajor, minMinor int
		want               bool
	}{
		"26.8.2 >= 20.11":  {"26.8.2", 20, 11, true},
		"22.0.0 >= 20.11":  {"22.0.0", 20, 11, true},
		"20.11.0 >= 20.11": {"20.11.0", 20, 11, true},
		"20.10.99 < 20.11": {"20.10.99", 20, 11, false},
		"18.20.0 < 20.11":  {"18.20.0", 20, 11, false},
		"v-prefixed":       {"v22.0.0", 20, 11, true},
		"string invalida":  {"abc", 20, 11, false},
		"vazio":            {"", 20, 11, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := NodeVersionAtLeast(tc.ver, tc.minMajor, tc.minMinor); got != tc.want {
				t.Errorf("NodeVersionAtLeast(%q, %d, %d) = %v, want %v",
					tc.ver, tc.minMajor, tc.minMinor, got, tc.want)
			}
		})
	}
}

func TestOpencodeVersionAtLeast(t *testing.T) {
	cases := map[string]struct {
		ver                          string
		minMajor, minMinor, minPatch int
		want                         bool
	}{
		"2.0.11 >= 2.0.0": {"2.0.11", 2, 0, 0, true},
		"2.1.0 >= 2.0.0":  {"2.1.0", 2, 0, 0, true},
		"3.0.0 >= 2.0.0":  {"3.0.0", 2, 0, 0, true},
		"1.18.27 < 2.0.0": {"1.18.27", 2, 0, 0, false},
		"2.0.0 == 2.0.0":  {"2.0.0", 2, 0, 0, true},
		"invalido":        {"", 2, 0, 0, false},
		"so major.minor":  {"2.0", 2, 0, 0, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := VersionAtLeast(tc.ver, tc.minMajor, tc.minMinor, tc.minPatch); got != tc.want {
				t.Errorf("VersionAtLeast(%q, %d, %d, %d) = %v, want %v",
					tc.ver, tc.minMajor, tc.minMinor, tc.minPatch, got, tc.want)
			}
		})
	}
}

func TestLocateOpencodePluginPackage(t *testing.T) {
	// Plugin presente no caminho do configDir.
	base := t.TempDir()
	pluginDir := filepath.Join(base, "plugins")
	if err := os.MkdirAll(filepath.Join(base, "node_modules", "@opencode", "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(base, "node_modules", "@opencode", "plugin", "package.json"),
		[]byte(`{"name":"@opencode/plugin","version":"2.0.11"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if path, ok := LocatePluginPackage(pluginDir); !ok || path == "" {
		t.Errorf("esperava achar @opencode/plugin em %s, got (%q, %v)", base, path, ok)
	}

	// Plugin ausente em todos os caminhos.
	base2 := t.TempDir()
	pluginDir2 := filepath.Join(base2, "plugins")
	t.Setenv("HOME", base2) // bloqueia ~/.opencode/node_modules
	if _, ok := LocatePluginPackage(pluginDir2); ok {
		t.Errorf("nao deveria achar @opencode/plugin em %s vazio", base2)
	}
}

func TestCheckOpencodeV2Runtime(t *testing.T) {
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	issues := CheckRuntime(pluginDir)
	for _, i := range issues {
		t.Logf("issue detectada: kind=%s detail=%s", i.Kind, i.Detail)
	}
}
