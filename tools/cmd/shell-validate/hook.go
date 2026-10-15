// PreToolUse hook handler. Reads a Claude/Codex/Cursor/Antigravity-style
// payload from stdin, extracts the bash command, runs the static validator,
// and writes back a hook response with `additionalContext` flagging the
// likely-invalid call. Never blocks the tool call — always writes a
// well-formed JSON response so a hook failure doesn't strand the parent
// CLI.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// passResponse retorna a resposta de "pass" adequada ao CLI de origem.
// Antigravity PreToolUse exige "decision" (allow/deny/ask); {} vazio
// e interpretado como deny e trava todas as tools (verificado em
// runtime 2026-09-22: matcher "*" dispara em view_file/run_command e
// {} bloqueava tudo). Claude/Codex/Cursor aceitam {} como pass.
func passResponse(raw []byte) string {
	// "toolCall":{ (com chave) = payload Antigravity real. Só a
	// substring "toolCall" também casa com `"toolCall":null` que o
	// json.Marshal emite para payloads Claude/Cursor sem ToolCall.
	if bytes.Contains(raw, []byte(`"toolCall":{`)) {
		return `{"decision":"allow"}`
	}
	return `{}`
}

// looksLikeObject distingue RawMessage ausente (vazio, "null", "[]", "")
// de RawMessage que contém um objeto JSON do qual vale a pena extrair
// `command`. Evita gastar unmarshal num campo que já é "null".
func looksLikeObject(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return strings.HasPrefix(s, "{")
}

// preToolUsePayload is the union of fields the 4 supported CLIs share for
// PreToolUse: session/conversation id, tool name, and a tool input blob
// whose shape varies by CLI. We pull the bash command out of the input
// in a best-effort way.
type preToolUsePayload struct {
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id"`
	SessionIDCamel string `json:"sessionId"`
	ToolName       string `json:"tool_name"`
	ToolCall       *struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// runHook lê um payload PreToolUse do stdin e escreve a resposta de hook no
// stdout. É sempre no-op quando AGENT_SYNC_PRETOOLUSE_VALIDATE!=1 — assim o
// binário pode ficar instalado e ativo sem impor custo na sessão.
func runHook(stdin io.Reader, stdout, stderr io.Writer) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintln(stderr, "shell-validate: read hook payload:", err)
		fmt.Fprint(stdout, "{}")
		return nil
	}
	// Estágio overwrite+cache (ADR-reuso-bash-sudo-agents §1), independente do
	// validador: se o comando é allowlistado e o cache está fresco, o resultado
	// já obtido volta como contexto e o agente não precisa re-executar.
	if cmdCacheEnabled() && serveCmdCacheHit(raw, stdout) {
		return nil
	}
	if strings.TrimSpace(os.Getenv("AGENT_SYNC_PRETOOLUSE_VALIDATE")) != "1" {
		fmt.Fprint(stdout, passResponse(raw))
		return nil
	}
	var payload preToolUsePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		fmt.Fprint(stdout, passResponse(raw))
		return nil
	}
	cmd, ok := extractBashCommand(payload)
	if !ok {
		fmt.Fprint(stdout, passResponse(raw))
		return nil
	}
	verdict := Classify(cmd)
	if verdict.Valid {
		fmt.Fprint(stdout, passResponse(raw))
		return nil
	}
	fmt.Fprintf(stdout, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"[agent-sync] shell-validate: %s. Verifique antes de executar."}}`,
		verdict.Reason)
	return nil
}

// serveCmdCacheHit injeta a saída cacheada como additionalContext quando o
// comando do payload é allowlistado e o cache está dentro do TTL. Devolve true
// quando já escreveu a resposta do hook.
//
// Antigravity (payload com "toolCall") só aceita o envelope `decision` em
// PreToolUse/PreInvocation — injetar additionalContext ali é lido como deny
// (mesmo achado do memory-nudge em 2026-09-22). Nesse caso respondemos
// {"decision":"allow"} e o cache segue disponível para os outros hosts.
func serveCmdCacheHit(raw []byte, stdout io.Writer) bool {
	var payload preToolUsePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	cmd, ok := extractBashCommand(payload)
	if !ok {
		return false
	}
	policy, ok := classifyCacheable(cmd)
	if !ok {
		return false
	}
	out, age, hit := cmdCacheGet(policy, cmd)
	if !hit {
		return false
	}
	if bytes.Contains(raw, []byte(`"toolCall":{`)) {
		fmt.Fprint(stdout, `{"decision":"allow"}`)
		return true
	}
	text, truncated := compressForContext(out)
	note := ""
	if truncated {
		note = " (truncado)"
	}
	ctx := fmt.Sprintf("[agent-sync] cmd-cache HIT `%s` (kind=%s, idade=%ds, TTL=%ds)%s — saída já obtida nesta sessão; não precisa executar de novo:\n\n%s",
		cmd, policy.kind, int(age.Seconds()), int(policy.ttl.Seconds()), note, text)
	resp, err := json.Marshal(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":     "PreToolUse",
		"additionalContext": ctx,
	}})
	if err != nil {
		return false
	}
	stdout.Write(resp)
	return true
}

// Suporta três formatos:
//  1. Claude Code / Codex: tool_name="Bash" e tool_input={"command": "..."}
//  2. Cursor: tool_name="Bash" e tool_input (mesma forma)
//  3. Antigravity: toolCall.name="Bash" e toolCall.args={"command": "..."}
//
// Quando tool_input/toolCall.args é um JSON-stringified (alguns adaptadores
// fazem isso), decodifica recursivamente uma vez.
func extractBashCommand(p preToolUsePayload) (string, bool) {
	name := p.ToolName
	if p.ToolCall != nil {
		name = p.ToolCall.Name
	}
	if name != "Bash" {
		return "", false
	}
	var raw json.RawMessage
	// Antigravity envia `tool_input: null` quando vazio, o que Unmarshal
	// converte para RawMessage("null") (4 bytes, não nil). Precisamos
	// distinguir "ausente" de "presente e vazio" — se não for objeto/array,
	// não tem `command` para extrair.
	switch {
	case p.ToolCall != nil && len(p.ToolCall.Args) > 0 && looksLikeObject(p.ToolCall.Args):
		raw = p.ToolCall.Args
	case len(p.ToolInput) > 0 && looksLikeObject(p.ToolInput):
		raw = p.ToolInput
	default:
		return "", false
	}
	// Tenta como string JSON-encoded.
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && asString != "" {
		raw = json.RawMessage(asString)
	}
	var obj struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil || strings.TrimSpace(obj.Command) == "" {
		return "", false
	}
	return obj.Command, true
}
