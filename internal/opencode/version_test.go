package opencode

import (
	"testing"
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
		if got := ParseMajor(in); got != want {
			t.Errorf("ParseMajor(%q)=%d want %d", in, got, want)
		}
	}
}

func TestOpencodeVersionOverride(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "2")
	if got := MajorVersion(); got != 2 {
		t.Errorf("override 2: got %d", got)
	}
	t.Setenv("AGENT_SYNC_OPENCODE_VERSION", "1")
	if got := MajorVersion(); got != 1 {
		t.Errorf("override 1: got %d", got)
	}
}

func TestOpencodePluginSource(t *testing.T) {
	if got := PluginSource("/base", "myplugin", 1); got != "/base/hooks/myplugin.opencode.ts" {
		t.Errorf("v1 source: got %q", got)
	}
	if got := PluginSource("/base", "myplugin", 2); got != "/base/hooks/myplugin.v2.ts" {
		t.Errorf("v2 source: got %q", got)
	}
}
