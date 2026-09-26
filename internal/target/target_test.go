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

	for _, expected := range []string{"claude", "codex", "antigravity", "opencode", "cursor"} {
		if !names[expected] {
			t.Errorf("target esperado ausente: %s", expected)
		}
	}
}
