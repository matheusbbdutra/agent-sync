package hooks

// pi_bridge.go: runtime bridge do Pi (`agent-sync hook pi --event=<evento>`).
//
// O Pi nao tem hooks declarativos. A extensao TS (hooks/pi-extension.ts) e um
// adapter fino que, a cada evento nativo, envia um payload JSON para este
// subcomando e aplica o resultado (bloquear tool call, injetar contexto,
// substituir output). Toda a decisao fica aqui, em Go (regra §4), reusando os
// mesmos scripts/binarios dos outros 5 hosts:
//
//	evento Pi            scripts/acoes
//	session_start        memory-prune-session-start, principles-inject
//	tool_call            secret-guard (deny), bash-rm-guardian, principles,
//	                     context-guard, memory-nudge, agent-react
//	tool_result          secret-guard (redact), memory-observe, docs-cache,
//	                     token-nudge (via budget.BuildTokenBudgetStatus)
//	agent_settled        memory-consolidate, ctx-window-summarize-at-stop
//	session_before_compact  state snapshot (actor pi)
//
// Fail-open e regra: qualquer erro vira `{}` e exit 0. Um hook nunca pode
// derrubar o turno do Pi.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/budget"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// piHookPayload e o contrato enviado pela extensao. Campos ausentes sao
// tratados como vazios (o Pi pode omitir tool_output em tool_call, por ex.).
type piHookPayload struct {
	Event          string         `json:"event"`
	SessionID      string         `json:"session_id"`
	Cwd            string         `json:"cwd"`
	Model          string         `json:"model"`
	TranscriptPath string         `json:"transcript_path"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
	ToolOutput     string         `json:"tool_output"`
	ContextTokens  int            `json:"context_tokens"`
	ContextWindow  int            `json:"context_window"`
}

// piHookResult e o contrato devolvido a extensao.
type piHookResult struct {
	Context string `json:"context,omitempty"`
	Block   bool   `json:"block,omitempty"`
	Reason  string `json:"reason,omitempty"`
	// Output substitui o resultado da tool (redacao do secret-guard). Vazio
	// significa "mantenha o output original".
	Output string `json:"output,omitempty"`
}

// piScriptTimeout limita cada script de hook. bash-rm-guardian roda
// repo-map --audit-removal (orçamento 5s no proprio script); os demais sao
// O(ms). Um timeout curto preserva a responsividade do turno.
const piScriptTimeout = 8 * time.Second

// piToolNameMap traduz os nomes de tool do Pi para o vocabulario
// compartilhado pelos hooks (Claude/Codex), que e o que secret-scan,
// memory-observe e docs-cache reconhecem.
var piToolNameMap = map[string]string{
	"bash":  "Bash",
	"read":  "Read",
	"write": "Write",
	"edit":  "Edit",
	"grep":  "Grep",
	"find":  "Glob",
	"ls":    "LS",
}

func piMapToolName(name string) string {
	if mapped, ok := piToolNameMap[strings.ToLower(name)]; ok {
		return mapped
	}
	return name
}

// runPiBridge implementa `agent-sync hook pi`. Nunca retorna erro: o contrato
// com a extensao e "sempre JSON, sempre exit 0".
func runPiBridge(args []string) error {
	fs := flag.NewFlagSet("agent-sync hook pi", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	eventFlag := fs.String("event", "", "evento Pi: session_start|tool_call|tool_result|agent_settled|session_before_compact|session_shutdown")
	if err := fs.Parse(args); err != nil {
		emitPiResult(piHookResult{})
		return nil
	}

	raw, _ := io.ReadAll(io.LimitReader(os.Stdin, 8<<20))
	var payload piHookPayload
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	event := strings.TrimSpace(*eventFlag)
	if event == "" {
		event = strings.TrimSpace(payload.Event)
	}
	if event == "" {
		event = "tool_call"
	}
	if payload.Cwd == "" {
		payload.Cwd, _ = os.Getwd()
	}

	exePath, _ := os.Executable()
	baseDir, err := pathutil.ResolveBaseDir(exePath, payload.Cwd, os.Getenv("AGENT_SYNC_HOME"))
	if err != nil {
		// Sem repo canonico nao ha scripts para rodar. Fail-open silencioso.
		emitPiResult(piHookResult{})
		return nil
	}

	emitPiResult(piDispatch(baseDir, event, payload))
	return nil
}

func emitPiResult(res piHookResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(res)
}

func piDispatch(baseDir, event string, p piHookPayload) piHookResult {
	switch event {
	case "session_start":
		return piSessionStart(baseDir, p)
	case "tool_call":
		return piToolCall(baseDir, p)
	case "tool_result":
		return piToolResult(baseDir, p)
	case "agent_settled":
		piAgentSettled(baseDir, p)
	case "session_before_compact":
		piPreCompact(baseDir, p)
	case "session_shutdown":
		piRunScript(baseDir, "memory-consolidate.stop.sh", p, piScriptTimeout)
	}
	return piHookResult{}
}

// piPayload monta o payload no formato Claude/Codex que os scripts ja
// entendem. `tool_response` carrega o output da tool no tool_result.
func piPayload(p piHookPayload, claudeEvent string) map[string]any {
	payload := map[string]any{
		"session_id":      p.SessionID,
		"cwd":             p.Cwd,
		"hook_event_name": claudeEvent,
		"tool_name":       piMapToolName(p.ToolName),
		"model":           p.Model,
		"transcript_path": p.TranscriptPath,
	}
	if p.ToolInput != nil {
		payload["tool_input"] = p.ToolInput
	}
	if p.ToolOutput != "" {
		payload["tool_response"] = p.ToolOutput
	}
	return payload
}

func piEnv(p piHookPayload) []string {
	env := []string{
		"AGENT_SYNC_AGENT_KIND=pi",
		"AGENT_SYNC_ACTOR=pi",
	}
	if p.Cwd != "" {
		env = append(env, "AGENT_SYNC_ROOT="+p.Cwd)
	}
	// O PATH da sessao pode resolver um agent-sync antigo (ex.: GOBIN do
	// mise). Prependemos o diretorio do binario em execucao para que os
	// scripts que chamam `agent-sync` (snapshot, budget) usem esta versao.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		env = append(env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return env
}

// piRunScript executa hooks/<script> com o payload JSON no stdin e devolve o
// stdout. Qualquer falha (script ausente, timeout, exit != 0) vira "".
func piRunScript(baseDir, script string, p piHookPayload, timeout time.Duration) string {
	path := filepath.Join(baseDir, "hooks", script)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	body, err := json.Marshal(piPayload(p, piClaudeEventFor(script)))
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Stdin = bytes.NewReader(body)
	cmd.Dir = p.Cwd
	cmd.Env = append(os.Environ(), piEnv(p)...)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return ""
	}
	return string(out)
}

// piClaudeEventFor devolve o hook_event_name esperado por cada script. So e
// relevante para os scripts que ecoam o evento no envelope; os demais o
// ignoram.
func piClaudeEventFor(script string) string {
	if strings.Contains(script, "posttooluse") {
		return "PostToolUse"
	}
	if strings.Contains(script, "stop.") || strings.HasPrefix(script, "agent-stop") {
		return "Stop"
	}
	return "PreToolUse"
}

// piParseEnvelope extrai contexto, bloqueio e redacao dos envelopes que os
// scripts cross-CLI emitem (Claude `hookSpecificOutput`, Antigravity
// `decision`/`injectSteps` e o `{}` neutro).
func piParseEnvelope(raw string) (contextText string, block bool, reason string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return "", false, ""
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return "", false, ""
	}
	if hso, ok := doc["hookSpecificOutput"].(map[string]any); ok {
		if s, ok := hso["additionalContext"].(string); ok && s != "" {
			contextText = s
		}
		if d, _ := hso["permissionDecision"].(string); d == "deny" {
			block = true
			reason, _ = hso["permissionDecisionReason"].(string)
		}
	}
	if d, _ := doc["decision"].(string); d == "deny" || d == "block" {
		block = true
		if r, ok := doc["reason"].(string); ok && r != "" {
			reason = r
		}
	}
	if steps, ok := doc["injectSteps"].([]any); ok {
		var msgs []string
		for _, step := range steps {
			if m, ok := step.(map[string]any); ok {
				if s, ok := m["message"].(string); ok && s != "" {
					msgs = append(msgs, s)
				}
			}
		}
		if len(msgs) > 0 {
			contextText = strings.Join(msgs, "\n")
		}
	}
	if s, ok := doc["additionalContext"].(string); ok && s != "" {
		contextText = s
	}
	return contextText, block, reason
}

func piSessionStart(baseDir string, p piHookPayload) piHookResult {
	// Auto-prune de scratch + staleness. Silencioso por contrato.
	piRunScript(baseDir, "memory-prune-session-start.sh", p, piScriptTimeout)
	// Premissas criticas (verdade absoluta + anti-overengineering), 1x por
	// sessao via dedup em $TMPDIR feito pelo proprio script.
	raw := piRunScript(baseDir, "principles-inject.pretooluse.sh", p, piScriptTimeout)
	text, _, _ := piParseEnvelope(raw)
	return piHookResult{Context: text}
}

func piToolCall(baseDir string, p piHookPayload) piHookResult {
	res := piHookResult{}
	var ctxParts []string

	// Ordem importa: secret-guard decide bloqueio antes de qualquer nudge.
	raw := piRunScript(baseDir, "secret-guard.pretooluse.sh", p, piScriptTimeout)
	text, block, reason := piParseEnvelope(raw)
	if block {
		return piHookResult{Block: true, Reason: reason}
	}
	if text != "" {
		ctxParts = append(ctxParts, text)
	}

	for _, script := range []string{
		"bash-rm-guardian.sh",
		"principles-inject.pretooluse.sh",
		"context-guard-nudge.pretooluse.sh",
		"memory-nudge.pretooluse.sh",
		"agent-react-nudge.pretooluse.sh",
	} {
		raw := piRunScript(baseDir, script, p, piScriptTimeout)
		t, b, r := piParseEnvelope(raw)
		if b {
			return piHookResult{Block: true, Reason: r}
		}
		if t != "" {
			ctxParts = append(ctxParts, t)
		}
	}
	if len(ctxParts) > 0 {
		res.Context = strings.Join(ctxParts, "\n")
	}
	return res
}

func piToolResult(baseDir string, p piHookPayload) piHookResult {
	res := piHookResult{}
	var ctxParts []string

	// Redacao de segredo no output (secret-scan redact). O envelope de
	// redacao usa additionalContext como texto substituto.
	raw := piRunScript(baseDir, "secret-guard.posttooluse.sh", p, piScriptTimeout)
	if text, _, _ := piParseEnvelope(raw); text != "" {
		res.Output = text
	}

	// Observacao continua de memoria e cache de docs (ambos silenciosos).
	piRunScript(baseDir, "memory-observe.posttooluse.sh", p, piScriptTimeout)
	if strings.Contains(strings.ToLower(p.ToolName), "fetch") || strings.Contains(strings.ToLower(p.ToolName), "mcp") {
		piRunScript(baseDir, "docs-cache.sh", p, piScriptTimeout)
	}

	if nudge := piTokenNudge(p); nudge != "" {
		ctxParts = append(ctxParts, nudge)
	}

	if len(ctxParts) > 0 {
		res.Context = strings.Join(ctxParts, "\n")
	}
	return res
}

// piTokenNudge usa o contrato compartilhado de budget, mas com os tokens de
// contexto que o Pi expoe nativamente (ctx.getContextUsage), em vez de exigir
// um transcript no formato do Claude.
func piTokenNudge(p piHookPayload) string {
	if p.ContextTokens <= 0 {
		return ""
	}
	threshold := 80
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_TOKEN_NUDGE_THRESHOLD")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threshold = n
		}
	}
	summaryPath := ""
	if p.Cwd != "" {
		summaryPath = filepath.Join(p.Cwd, ".agent-sync", "summary.md")
	}
	status, err := budget.BuildTokenBudgetStatus("pi", p.SessionID, p.Model,
		p.ContextWindow, p.ContextTokens, 0, 0, summaryPath, threshold, nil)
	if err != nil || !status.ShouldNudge {
		return ""
	}
	if !piNudgeOnce(status.SessionID, status.Trigger) {
		return ""
	}
	return budget.NudgeMessage(status)
}

// piNudgeOnce garante um aviso por sessao+gatilho: o nudge roda a cada tool
// result, e sem o marker a mesma mensagem se repete durante toda a sessao.
// Fail-open: se o marker nao puder ser criado, o aviso e emitido.
func piNudgeOnce(sessionID, trigger string) bool {
	dir := filepath.Join(os.TempDir(), "agent-sync-token-nudge")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return true
	}
	name := piSlug(sessionID) + "." + piSlug(trigger) + ".seen"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// piSlug normaliza componentes de nome de arquivo (sessao/gatilho vem do host).
func piSlug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func piAgentSettled(baseDir string, p piHookPayload) {
	// Consolidacao de memoria da sessao + auto-summarize (background no
	// proprio script). Nao bloqueiam o settle.
	piRunScript(baseDir, "memory-consolidate.stop.sh", p, piScriptTimeout)
	piRunScript(baseDir, "ctx-window-summarize-at-stop.sh", p, piScriptTimeout)
}

func piPreCompact(baseDir string, p piHookPayload) {
	// Snapshot estruturado do PreCompact (ADR-precompact-snapshot). Usa o
	// script canonico com o actor corrigido para pi.
	piRunScript(baseDir, "agent-sync-precompact-snapshot.sh", p, piScriptTimeout)
}
