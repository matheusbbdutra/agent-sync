package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPiMapToolName(t *testing.T) {
	cases := map[string]string{
		"bash":        "Bash",
		"read":        "Read",
		"write":       "Write",
		"edit":        "Edit",
		"grep":        "Grep",
		"find":        "Glob",
		"ls":          "LS",
		"custom_tool": "custom_tool",
	}
	for in, want := range cases {
		if got := piMapToolName(in); got != want {
			t.Errorf("piMapToolName(%q) = %q, esperado %q", in, got, want)
		}
	}
}

func TestPiParseEnvelope(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantCtx    string
		wantBlock  bool
		wantReason string
	}{
		{"vazio", "{}", "", false, ""},
		{"claude context", `{"hookSpecificOutput":{"additionalContext":"nudge"}}`, "nudge", false, ""},
		{"claude deny", `{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"secret"}}`, "", true, "secret"},
		{"antigravity deny", `{"decision":"deny","reason":"blocked"}`, "", true, "blocked"},
		{"injectSteps", `{"decision":"allow","injectSteps":[{"label":"x","message":"warn"}]}`, "warn", false, ""},
		{"invalido", `not-json`, "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, block, reason := piParseEnvelope(tc.raw)
			if ctx != tc.wantCtx || block != tc.wantBlock || reason != tc.wantReason {
				t.Errorf("piParseEnvelope(%q) = (%q,%v,%q), esperado (%q,%v,%q)",
					tc.raw, ctx, block, reason, tc.wantCtx, tc.wantBlock, tc.wantReason)
			}
		})
	}
}

// fakeScript cria hooks/<name> em um baseDir temporário.
func fakeScript(t *testing.T, baseDir, name, body string) {
	t.Helper()
	dir := filepath.Join(baseDir, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\ncat >/dev/null\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPiDispatchToolCallSecretGuardBlock(t *testing.T) {
	baseDir := t.TempDir()
	fakeScript(t, baseDir, "secret-guard.pretooluse.sh",
		`printf '%s' '{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"x"}}'`)

	res := piDispatch(baseDir, "tool_call", piHookPayload{
		SessionID: "s1",
		Cwd:       baseDir,
		ToolName:  "read",
		ToolInput: map[string]any{"path": "/tmp/.env"},
	})
	if !res.Block {
		t.Fatalf("esperava block=true, obteve %+v", res)
	}
}

func TestPiDispatchSessionStartInjectaPrinciples(t *testing.T) {
	baseDir := t.TempDir()
	fakeScript(t, baseDir, "principles-inject.pretooluse.sh",
		`printf '%s' '{"hookSpecificOutput":{"additionalContext":"PRINCIPLES"}}'`)

	res := piDispatch(baseDir, "session_start", piHookPayload{SessionID: "s2", Cwd: baseDir})
	if res.Context != "PRINCIPLES" {
		t.Fatalf("esperava contexto PRINCIPLES, obteve %q", res.Context)
	}
}

func TestPiDispatchToolResultRedacao(t *testing.T) {
	baseDir := t.TempDir()
	fakeScript(t, baseDir, "secret-guard.posttooluse.sh",
		`printf '%s' '{"hookSpecificOutput":{"additionalContext":"<REDACTED:X>"}}'`)

	res := piDispatch(baseDir, "tool_result", piHookPayload{
		SessionID:  "s3",
		Cwd:        baseDir,
		ToolName:   "read",
		ToolOutput: "API_KEY=abc",
	})
	if res.Output != "<REDACTED:X>" {
		t.Fatalf("esperava output redigido, obteve %q", res.Output)
	}
}

func TestPiTokenNudgeAcimaDoThreshold(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // isola o marker de nudge por sessao
	got := piTokenNudge(piHookPayload{
		SessionID:     "s4",
		Model:         "claude-sonnet-4-5",
		ContextTokens: 190_000,
	})
	if got == "" {
		t.Fatal("esperava nudge para 190k tokens em janela 200k")
	}
	if empty := piTokenNudge(piHookPayload{ContextTokens: 0}); empty != "" {
		t.Fatalf("sem tokens deveria ser no-op, obteve %q", empty)
	}
}

func TestPiNudgeUmaVezPorSessaoEGatilho(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, ".agent-sync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	summary := filepath.Join(dir, "summary.md")
	if err := os.WriteFile(summary, []byte("# handoff\n"), 0o644); err != nil {
		t.Fatalf("summary: %v", err)
	}
	old := time.Now().Add(-6 * time.Hour)
	if err := os.Chtimes(summary, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	p := piHookPayload{SessionID: "sess-once", Cwd: root, Model: "m",
		ContextTokens: 1000, ContextWindow: 1000000}
	first := piTokenNudge(p)
	if !strings.Contains(first, "Handoff desatualizado") {
		t.Fatalf("esperava motivo de handoff, veio %q", first)
	}
	if second := piTokenNudge(p); second != "" {
		t.Errorf("segundo nudge deveria ser silencioso, veio %q", second)
	}
}

func TestPiNudgePercentualApenasNoGatilhoDeTokens(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, ".agent-sync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.md"), []byte("# handoff\n"), 0o644); err != nil {
		t.Fatalf("summary: %v", err)
	}

	p := piHookPayload{SessionID: "sess-pct", Cwd: root, Model: "m",
		ContextTokens: 900000, ContextWindow: 1000000}
	msg := piTokenNudge(p)
	if !strings.Contains(msg, "90% da janela") {
		t.Errorf("esperava percentual real no gatilho de tokens, veio %q", msg)
	}
}

func TestPiPayloadIncluiToolResponse(t *testing.T) {
	payload := piPayload(piHookPayload{
		SessionID:  "s5",
		Cwd:        "/tmp",
		ToolName:   "read",
		ToolInput:  map[string]any{"path": "/tmp/x"},
		ToolOutput: "conteudo",
	}, "PostToolUse")
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["tool_name"] != "Read" {
		t.Errorf("tool_name = %v, esperado Read", got["tool_name"])
	}
	if got["tool_response"] != "conteudo" {
		t.Errorf("tool_response = %v, esperado conteudo", got["tool_response"])
	}
}
