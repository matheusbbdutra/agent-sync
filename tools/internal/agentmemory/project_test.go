package agentmemory

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveOriginUsesSameGitIDAcrossPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	roots := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")}
	var ids []string
	for i, root := range roots {
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", root}, {"-C", root, "remote", "add", "origin", []string{"https://example.org/team/repo.git", "git@example.org:team/repo.git"}[i]}} {
			if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
				t.Fatalf("git: %v: %s", err, output)
			}
		}
		origin, err := ResolveOrigin(root)
		if err != nil {
			t.Fatal(err)
		}
		if origin.ProjectPath != root || origin.PC == "" {
			t.Fatalf("origem inválida: %+v", origin)
		}
		ids = append(ids, origin.ProjectID)
	}
	if ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("IDs distintos para mesmo remoto: %q %q", ids[0], ids[1])
	}
}

func TestResolveOriginUsesConfiguredIDWithoutRemote(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	if _, err := ResolveOrigin(root); err == nil {
		t.Fatal("projeto sem remoto nem ID aceito")
	}
	path, err := EnsureConfig()
	if err != nil {
		t.Fatal(err)
	}
	config := Config{Projects: map[string]string{root: "shared-project"}}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	origin, err := ResolveOrigin(root)
	if err != nil || origin.ProjectID != "shared-project" {
		t.Fatalf("ID configurado não usado: %+v %v", origin, err)
	}
}
