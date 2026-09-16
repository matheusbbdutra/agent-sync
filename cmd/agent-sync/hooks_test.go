package main

import "testing"

func TestAdaptCodexProtectionHooks(t *testing.T) {
	entries := []hookEntry{{Hooks: []hookCmd{
		{Command: "npx protect-mcp@0.7.4 evaluate --policy policy.cedar"},
		{Command: "if [ -f flag ]; then exit 0; fi; npx protect-mcp@0.7.4 sign --tool \"$TOOL_NAME\""},
	}}}

	adapted := adaptCodexProtectionHooks(entries, "/repo/hooks/codex-protect-mcp-adapter.sh")
	if adapted[0].Hooks[0].Command != "/repo/hooks/codex-protect-mcp-adapter.sh evaluate --policy policy.cedar" {
		t.Fatalf("evaluate não adaptado: %q", adapted[0].Hooks[0].Command)
	}
	if adapted[0].Hooks[1].Command != "if [ -f flag ]; then exit 0; fi; /repo/hooks/codex-protect-mcp-adapter.sh sign --tool \"$TOOL_NAME\"" {
		t.Fatalf("sign não adaptado: %q", adapted[0].Hooks[1].Command)
	}
}
