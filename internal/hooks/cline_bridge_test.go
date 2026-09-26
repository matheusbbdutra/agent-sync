package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeHook cria um script em <baseDir>/hooks/<name> com modo 0755.
func writeFakeHook(t *testing.T, baseDir, name, body string) {
	t.Helper()
	dir := filepath.Join(baseDir, "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizedClinePayloadPreToolUse(t *testing.T) {
	raw := []byte(`{
		"taskId": "ses-cline-1",
		"hookName": "tool_call",
		"iteration": 3,
		"workspaceRoots": ["/tmp/proj"],
		"tool_call": {"id": "c1", "name": "execute_command", "input": {"command": "rm -rf tools/cmd/memory-mcp"}},
		"preToolUse": {"toolName": "execute_command", "parameters": "{\"command\":\"rm -rf tools/cmd/memory-mcp\"}"}
	}`)

	norm, root, sessionID, err := normalizedClinePayload("PreToolUse", raw)
	if err != nil {
		t.Fatalf("normalizedClinePayload: %v", err)
	}
	if sessionID != "ses-cline-1" {
		t.Errorf("session_id esperado ses-cline-1, obteve %q", sessionID)
	}
	if root != "/tmp/proj" {
		t.Errorf("root esperado /tmp/proj, obteve %q", root)
	}

	var got claudeHookPayload
	if err := json.Unmarshal(norm, &got); err != nil {
		t.Fatalf("payload normalizado invalido: %v", err)
	}
	if got.SessionID != "ses-cline-1" {
		t.Errorf("session_id=%q", got.SessionID)
	}
	if got.HookEventName != "PreToolUse" {
		t.Errorf("hook_event_name=%q", got.HookEventName)
	}
	if got.ToolName != "execute_command" {
		t.Errorf("tool_name=%q", got.ToolName)
	}
	if !strings.Contains(string(got.ToolInput), "rm -rf tools/cmd/memory-mcp") {
		t.Errorf("tool_input sem command: %s", got.ToolInput)
	}
	if got.AgentKind != "cline" {
		t.Errorf("agent_kind=%q", got.AgentKind)
	}
}

func TestNormalizedClinePayloadFallbackParametersString(t *testing.T) {
	// Payload só com o bloco do evento (parameters como string JSON) e sem
	// taskId: session_id deve cair no rootSessionId.
	raw := []byte(`{
		"sessionContext": {"rootSessionId": "root-1"},
		"postToolUse": {"toolName": "write_to_file", "parameters": "{\"path\":\"a.md\"}", "result": "ok", "success": true}
	}`)

	norm, _, sessionID, err := normalizedClinePayload("PostToolUse", raw)
	if err != nil {
		t.Fatalf("normalizedClinePayload: %v", err)
	}
	if sessionID != "root-1" {
		t.Errorf("session_id esperado root-1, obteve %q", sessionID)
	}

	var got claudeHookPayload
	if err := json.Unmarshal(norm, &got); err != nil {
		t.Fatal(err)
	}
	if got.HookEventName != "PostToolUse" {
		t.Errorf("hook_event_name=%q", got.HookEventName)
	}
	if got.ToolName != "write_to_file" {
		t.Errorf("tool_name=%q", got.ToolName)
	}
	if !strings.Contains(string(got.ToolInput), `"path":"a.md"`) {
		t.Errorf("tool_input nao decodificou parameters: %s", got.ToolInput)
	}
	if string(got.ToolResponse) != `"ok"` {
		t.Errorf("tool_response=%s", got.ToolResponse)
	}
}

func TestNormalizedClinePayloadEventosDeCiclo(t *testing.T) {
	cases := []struct{ cline, claude string }{
		{"TaskStart", "SessionStart"},
		{"TaskResume", "SessionStart"},
		{"TaskComplete", "Stop"},
		{"TaskError", "Stop"},
		{"PreToolUse", "PreToolUse"},
	}
	for _, tc := range cases {
		if got := claudeEventForClineEvent(tc.cline); got != tc.claude {
			t.Errorf("claudeEventForClineEvent(%s)=%s, esperado %s", tc.cline, got, tc.claude)
		}
	}
}

func TestNormalizedClinePayloadPromptSubmit(t *testing.T) {
	raw := []byte(`{"taskId":"s1","userPromptSubmit":{"prompt":"faca X"}}`)
	norm, _, _, err := normalizedClinePayload("UserPromptSubmit", raw)
	if err != nil {
		t.Fatal(err)
	}
	var got claudeHookPayload
	if err := json.Unmarshal(norm, &got); err != nil {
		t.Fatal(err)
	}
	if got.Prompt != "faca X" {
		t.Errorf("prompt=%q", got.Prompt)
	}
}

func TestNormalizedClinePayloadInvalido(t *testing.T) {
	if _, _, _, err := normalizedClinePayload("PreToolUse", []byte("{nao-e-json")); err == nil {
		t.Fatal("esperava erro para payload invalido")
	}
	// Payload vazio nao é erro: session_id default (hook pode rodar sem payload).
	_, _, sessionID, err := normalizedClinePayload("PreToolUse", nil)
	if err != nil {
		t.Fatalf("payload vazio deveria ser tolerado: %v", err)
	}
	if sessionID != "default" {
		t.Errorf("session_id=%q", sessionID)
	}
}

func TestTranslateClaudeHookOutput(t *testing.T) {
	cases := []struct {
		name      string
		out       string
		denyMode  string
		wantCtx   string
		wantCance bool
		wantReas  string
	}{
		{
			name:    "hookSpecificOutput.additionalContext",
			out:     `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"nudge A"}}`,
			wantCtx: "nudge A",
		},
		{
			name:    "additionalContext top-level",
			out:     `{"additionalContext":"nudge B"}`,
			wantCtx: "nudge B",
		},
		{
			name:    "systemMessage",
			out:     `{"systemMessage":"aviso C"}`,
			wantCtx: "aviso C",
		},
		{
			name:    "errorMessage vira contexto (paridade com adapter do Cline)",
			out:     `{"errorMessage":"falhou D"}`,
			wantCtx: "falhou D",
		},
		{
			name:      "permissionDecision deny com deny-mode stop",
			out:       `{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"segredo E"}}`,
			denyMode:  "stop",
			wantCance: true,
			wantReas:  "segredo E",
		},
		{
			name:     "permissionDecision deny com deny-mode warn",
			out:      `{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"segredo F"}}`,
			denyMode: "warn",
			wantCtx:  "[blocked by hook] segredo F",
		},
		{
			name:     "decision block com deny-mode warn",
			out:      `{"decision":"block","reason":"bloqueio G"}`,
			denyMode: "warn",
			wantCtx:  "[blocked by hook] bloqueio G",
		},
		{
			name:    "cancel explícito do contrato Cline (mesmo com deny-mode warn)",
			out:     `{"cancel":false,"context":"ctx H"}`,
			wantCtx: "ctx H",
		},
		{
			name:      "cancel true do contrato Cline",
			out:       `{"cancel":true,"cancelReason":"abort I"}`,
			denyMode:  "warn",
			wantCance: true,
			wantReas:  "abort I",
		},
		{
			name: "stdout vazio",
			out:  "",
		},
		{
			name: "stdout nao-JSON",
			out:  "bash: erro qualquer",
		},
		{
			name:    "JSON na ultima linha (stdout com ruido)",
			out:     "warning: algo\n{\"context\":\"ctx J\"}\n",
			wantCtx: "ctx J",
		},
	}

	for _, tc := range cases {
		denyMode := tc.denyMode
		if denyMode == "" {
			denyMode = "stop"
		}
		ctx, cancel, reason := translateClaudeHookOutput([]byte(tc.out), denyMode)
		if ctx != tc.wantCtx {
			t.Errorf("%s: ctx=%q, esperado %q", tc.name, ctx, tc.wantCtx)
		}
		if cancel != tc.wantCance {
			t.Errorf("%s: cancel=%v, esperado %v", tc.name, cancel, tc.wantCance)
		}
		if reason != tc.wantReas {
			t.Errorf("%s: reason=%q, esperado %q", tc.name, reason, tc.wantReas)
		}
	}
}

func TestMergeClineHookResults(t *testing.T) {
	merged := mergeClineHookResults([]clineHookResult{
		{contexts: []string{"A"}},
		{contexts: []string{"", "B"}},
		{contexts: []string{"A"}}, // duplicado
		{cancel: true, reason: "motivo X"},
	})
	if merged.cancel != true {
		t.Errorf("cancel deveria propagar")
	}
	if merged.reason != "motivo X" {
		t.Errorf("reason=%q", merged.reason)
	}
	rendered := string(renderClineHookResponse(merged))
	if !strings.Contains(rendered, `"context":"A\nB"`) {
		t.Errorf("contexto mergeado incorreto: %s", rendered)
	}
}

func TestRenderClineHookResponseTruncaContexto(t *testing.T) {
	big := strings.Repeat("x", maxClineHookContext+100)
	out := renderClineHookResponse(clineHookResult{contexts: []string{big}})
	if len(out) > maxClineHookContext+200 {
		t.Fatalf("resposta nao truncada: %d bytes", len(out))
	}
	if !strings.Contains(string(out), "hook context truncated") {
		t.Errorf("faltou marcador de truncamento")
	}
}

func TestRenderClineHookResponseVazio(t *testing.T) {
	out := renderClineHookResponse(clineHookResult{contexts: []string{}})
	var decoded clineHookResponse
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("resposta nao e JSON valido: %s", out)
	}
	if decoded.Cancel || decoded.Context != "" {
		t.Errorf("esperava no-op, obteve %s", out)
	}
}

// TestRunClineBridgeComScriptsFake valida o fluxo completo da ponte com
// scripts fake que emulam o contrato Claude/Codex — inclui a captura do
// payload normalizado para provar que a tradução chegou ao script.
func TestRunClineBridgeComScriptsFake(t *testing.T) {
	baseDir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "payload.json")
	t.Setenv("FAKE_CAPTURE", capture)

	writeFakeHook(t, baseDir, "memory-nudge.pretooluse.sh", `#!/usr/bin/env bash
set -euo pipefail
cat > "$FAKE_CAPTURE"
printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"nudge #25"}}'
`)
	writeFakeHook(t, baseDir, "secret-guard.pretooluse.sh", `#!/usr/bin/env bash
printf '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`)
	// PostToolUse do mesmo diretório deve ser ignorado neste evento.
	writeFakeHook(t, baseDir, "memory-observe.posttooluse.sh", `#!/usr/bin/env bash
printf '{"additionalContext":"NUNCA_DEVE_APARECER"}'
`)

	payload := `{"taskId":"ses-42","workspaceRoots":["/tmp/proj"],
		"tool_call":{"id":"c1","name":"execute_command","input":{"command":"ls -la"}},
		"preToolUse":{"toolName":"execute_command"}}`

	var stdout, stderr bytes.Buffer
	err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + baseDir},
		strings.NewReader(payload), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunClineBridge: %v (stderr=%s)", err, stderr.String())
	}

	out := stdout.String()
	if strings.Contains(out, "NUNCA_DEVE_APARECER") {
		t.Errorf("script de PostToolUse rodou em PreToolUse: %s", out)
	}
	var resp clineHookResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout nao e JSON: %s", out)
	}
	if resp.Cancel {
		t.Errorf("cancel deveria ser false: %s", out)
	}
	if !strings.Contains(resp.Context, "nudge #25") {
		t.Errorf("contexto esperado do nudge: %s", out)
	}

	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("script fake nao capturou payload: %v", err)
	}
	var got claudeHookPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("payload capturado invalido: %v", err)
	}
	if got.SessionID != "ses-42" {
		t.Errorf("session_id=%q", got.SessionID)
	}
	if got.ToolName != "execute_command" {
		t.Errorf("tool_name=%q", got.ToolName)
	}
	if !strings.Contains(string(got.ToolInput), "ls -la") {
		t.Errorf("tool_input=%s", got.ToolInput)
	}
}

func TestRunClineBridgeDenyMode(t *testing.T) {
	baseDir := t.TempDir()
	writeFakeHook(t, baseDir, "secret-guard.pretooluse.sh", `#!/usr/bin/env bash
printf '{"hookSpecificOutput":{"permissionDecision":"deny","permissionDecisionReason":"segredo detectado"}}'
`)

	var stdout, stderr bytes.Buffer
	if err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + baseDir},
		strings.NewReader(`{"taskId":"s1"}`), &stdout, &stderr); err != nil {
		t.Fatalf("RunClineBridge: %v", err)
	}
	var resp clineHookResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Cancel {
		t.Errorf("deny-mode default (stop) deveria cancelar: %s", stdout.String())
	}
	if resp.CancelReason != "segredo detectado" {
		t.Errorf("cancelReason=%q", resp.CancelReason)
	}

	stdout.Reset()
	if err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + baseDir, "--deny-mode=warn"},
		strings.NewReader(`{"taskId":"s1"}`), &stdout, &stderr); err != nil {
		t.Fatalf("RunClineBridge: %v", err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Cancel {
		t.Errorf("deny-mode=warn nao deveria cancelar: %s", stdout.String())
	}
	if !strings.Contains(resp.Context, "[blocked by hook] segredo detectado") {
		t.Errorf("contexto sem aviso de bloqueio: %s", stdout.String())
	}
}

func TestRunClineBridgeScriptFalhandoNaoDerruba(t *testing.T) {
	baseDir := t.TempDir()
	writeFakeHook(t, baseDir, "memory-nudge.pretooluse.sh", `#!/usr/bin/env bash
echo "erro proposital" >&2
exit 3
`)
	writeFakeHook(t, baseDir, "context-guard-nudge.pretooluse.sh", `#!/usr/bin/env bash
printf '{"hookSpecificOutput":{"additionalContext":"contexto sobrevive"}}'
`)

	var stdout, stderr bytes.Buffer
	if err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + baseDir},
		strings.NewReader(`{"taskId":"s1"}`), &stdout, &stderr); err != nil {
		t.Fatalf("RunClineBridge: %v", err)
	}
	var resp clineHookResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout nao e JSON: %s", stdout.String())
	}
	if resp.Cancel {
		t.Errorf("falha de script nao deve cancelar: %s", stdout.String())
	}
	if !strings.Contains(resp.Context, "contexto sobrevive") {
		t.Errorf("script seguinte ao que falhou nao rodou: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "memory-nudge") {
		t.Errorf("falha nao logada em stderr: %s", stderr.String())
	}
}

func TestRunClineBridgePayloadInvalidoRespondeNoOp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := RunClineBridge(
		[]string{"--event=PreToolUse", "--base-dir=" + t.TempDir()},
		strings.NewReader("{nao-e-json"), &stdout, &stderr); err != nil {
		t.Fatalf("RunClineBridge: %v", err)
	}
	var resp clineHookResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("stdout precisa ser JSON mesmo com payload invalido: %s", stdout.String())
	}
	if resp.Cancel || resp.Context != "" {
		t.Errorf("esperava no-op: %s", stdout.String())
	}
}

func TestRunClineBridgeSemEvento(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := RunClineBridge(nil, strings.NewReader("{}"), &stdout, &stderr)
	if err == nil {
		t.Fatal("esperava erro sem --event")
	}
	if !strings.Contains(err.Error(), "--event") {
		t.Errorf("erro inesperado: %v", err)
	}
}
