package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTranscript(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunHookFlagsUnverifiedSuccess(t *testing.T) {
	entry := `{"message":{"role":"assistant","content":[{"type":"text","text":"Pronto, funcionalidade implementada com sucesso."}]}}`
	path := writeTranscript(t, []string{entry})
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("additionalContext")) {
		t.Errorf("expected flagged nudge, got: %s", out.String())
	}
}

func TestRunHookIgnoresVerifiedSuccess(t *testing.T) {
	entry := `{"message":{"role":"assistant","content":[{"type":"text","text":"Implementado, ver main.go:10 e o teste passou (exit code 0)."}]}}`
	path := writeTranscript(t, []string{entry})
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response, got: %s", out.String())
	}
}

func TestRunHookHandlesMissingTranscript(t *testing.T) {
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: "/nonexistent/path.jsonl"})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response for missing transcript, got: %s", out.String())
	}
}

func TestRunHookIgnoresOldToolErrorWhenCurrentTurnClean(t *testing.T) {
	lines := []string{
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"vou tentar de novo"}]}}`,
		`{"message":{"role":"user","content":[{"type":"text","text":"agora tenta de novo"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (old tool error must not persist), got: %s", out.String())
	}
}

func TestRunHookStillFlagsCurrentTurnError(t *testing.T) {
	lines := []string{
		`{"message":{"role":"user","content":[{"type":"text","text":"implementa X"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","is_error":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("additionalContext")) {
		t.Errorf("expected flagged nudge for current-turn tool error, got: %s", out.String())
	}
}

func TestRunHookResetsOnUserTextMixedWithToolResult(t *testing.T) {
	lines := []string{
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"user","content":[{"type":"text","text":"tenta de novo"},{"type":"tool_result","is_error":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"implementado com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (mixed text+tool_result must still reset), got: %s", out.String())
	}
}

func TestRunHookIgnoresExitCode1WithoutIsError(t *testing.T) {
	lines := []string{
		`{"message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"grep -r foo nonexistent"}},{"type":"tool_use","name":"Bash","input":{"command":"diff a b"}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":false,"content":"(no output)\nexit code 1"}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":false,"content":"(no diff)\nexit code 1"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"grep e diff retornaram sem match, segue o fluxo"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"pronto, validado e funcionando"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (grep/diff exit 1 com isError:false nao deve flaguear), got: %s", out.String())
	}
}

func TestRunHookAcceptsCursorShellAsVerification(t *testing.T) {
	// Cursor registra a tool como "Shell" (não "Bash"). Sem reconhecer "shell",
	// alegação com evidência no mesmo turno era falso-positivo (SMOKE-TEST-R C2).
	lines := []string{
		`{"role":"user","message":{"role":"user","content":[{"type":"text","text":"Liste os arquivos Python com evidência"}]}}`,
		`{"role":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Shell","input":{"command":"find . -name '*.py'"}}]}}`,
		`{"role":"user","message":{"role":"user","content":[{"type":"tool_result","is_error":false,"content":"./hooks/docs-cache.py"}]}}`,
		`{"role":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Pronto. Há 1 arquivo. Ver hooks/docs-cache.py:1 e o comando passou."}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (Cursor Shell = verificação), got: %s", out.String())
	}
}

func TestRunHookStillFlagsToolStatusFailedMarker(t *testing.T) {
	lines := []string{
		`{"message":{"role":"user","content":[{"type":"text","text":"faz X"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","is_error":false,"content":"[TOOL_STATUS: FAILED] wrapper reportou falha"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"pronto, feito com sucesso"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("additionalContext")) {
		t.Errorf("expected flagged nudge para [TOOL_STATUS: FAILED] mesmo com isError:false, got: %s", out.String())
	}
}

// Regressão real (2026-09-19): ler o próprio código-fonte do guard via Read
// (que cita "[TOOL_STATUS: FAILED]" como documentação/regex/fixture) não pode
// ser tratado como evidência de falha — Read é somente-leitura e nunca emite
// esse marcador de verdade. O bug original também nunca resetava por causa do
// prompt de usuário em texto puro (content como string) ser descartado pelo
// parser antigo; este teste cobre ambos os pontos numa única transcrição.
func TestRunHookIgnoresToolStatusMarkerFromReadOnlyTool(t *testing.T) {
	lines := []string{
		`{"message":{"role":"user","content":"investiga o bug do hook"}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"detector.go"}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","is_error":false,"content":"HasToolError bool // [TOOL_STATUS: FAILED]"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"corrigido e pronto"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("erro não tratado")) {
		t.Errorf("Read citando o marcador não deveria contar como erro de ferramenta, got: %s", out.String())
	}
}

func TestRunHookAcceptsCompactAsVerification(t *testing.T) {
	// A-26: hook PreCompact (syncContextSnapshotHook wirado em A-14) registra
	// tool_use "compact" no transcript Claude Code quando ocorre compactacao.
	// Sem reconhecer essa tool, alegacao de sucesso no mesmo turno vira
	// falso-positivo (analogia direta ao fix D-25 para Shell do Cursor).
	lines := []string{
		`{"message":{"role":"user","content":[{"type":"text","text":"trabalha em X ate compactar"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"compact","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","is_error":false,"content":"session compacted"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"Pronto. Concluído, compactação registrada."}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (compact = verificacao estrutural), got: %s", out.String())
	}
}

// Regressão real: content como string simples (prompt digitado sem anexos,
// shape real do Claude Code 2.1.275) precisa resetar o turno como um bloco
// de texto em array resetaria — antes, o Unmarshal falhava e a linha inteira
// era descartada, quebrando o janelamento por turno silenciosamente.
func TestRunHookResetsOnPlainStringUserContent(t *testing.T) {
	lines := []string{
		`{"message":{"role":"user","content":[{"type":"tool_result","is_error":true,"content":"exit code 1"}]}}`,
		`{"message":{"role":"user","content":"agora tenta de novo"}}`,
		`{"message":{"role":"assistant","content":[{"type":"tool_use","id":"tu2","name":"Bash","input":{}}]}}`,
		`{"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu2","is_error":false,"content":"exit code 0"}]}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"feito com sucesso, `+"`go test`"+` passou"}]}}`,
	}
	path := writeTranscript(t, lines)
	payload, _ := json.Marshal(stopPayload{SessionID: "s1", TranscriptPath: path})

	var out, errOut bytes.Buffer
	if err := runHook(bytes.NewReader(payload), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "{}" {
		t.Errorf("expected empty hook response (erro antigo resetado por prompt em string simples), got: %s", out.String())
	}
}
