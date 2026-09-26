package hooks

// cline_bridge.go: ponte de hooks entre o contrato do Cline v3 e os scripts
// agent-sync (A-80.1).
//
// Cline descobre hooks por ARQUIVO em `~/.cline/hooks/<EventName>[.<ext>]`
// (a extensão é opcional; o nome-base precisa casar com o enum de eventos).
// O script recebe o payload JSON no stdin e devolve JSON no stdout — o stdout
// inteiro precisa ser um único JSON (ou a última linha `HOOK_CONTROL\t<json>`).
// Contrato descoberto empiricamente no binário cline 3.0.65 em 2026-09-25 e
// documentado em docs/investigations/cline-hooks-contract.md.
//
// Campos de resposta aceitos pelo Cline:
//
//	cancel (bool), cancelReason (string), context|contextModification (string),
//	errorMessage (string), review (bool), overrideInput (any)
//
// `cancel:true` => Cline executa applyStopControl() => throw, ou seja ABORTA o
// run (é o único primitivo de bloqueio de hooks de arquivo; não existe deny
// por-tool). Por isso `--deny-mode` (default `stop`) decide o que fazer quando
// um script agent-sync responde no contrato Claude/Codex com
// `permissionDecision=deny`: `stop` converte em cancel (fail-safe, aborta o
// run) e `warn` apenas injeta o motivo como contexto.
//
// Os hooks agent-sync falam o contrato Claude/Codex
// (`hookSpecificOutput.additionalContext` / `permissionDecision`). Esta ponte:
//
//  1. lê o payload Cline do stdin;
//  2. normaliza para um payload Claude-compatível (session_id, tool_name,
//     tool_input, tool_response, hook_event_name, cwd);
//  3. executa os scripts agent-sync mapeados para o evento Cline;
//  4. traduz e mergeia as saídas no contrato Cline.
//
// Origem: A-80 (ADR-cline-hooks-mcp-wiramento.md), A-79 (wiramento parcial).
// Zero-LLM (D-6). Wiramento dos shims: internal/hooks/apply_cline.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxClineHookContext espelha o limite do Cline (50000 chars): contexto
	// acima disso é truncado pelo próprio Cline. Truncamos antes para manter
	// o JSON previsível.
	maxClineHookContext = 50000

	// defaultClineHookTimeout alinha com o `timeout: 10` dos hooks wirados
	// nas outras CLIs.
	defaultClineHookTimeout = 10 * time.Second

	// maxClineHookPayload limita o stdin aceito (defesa contra payload gigante).
	maxClineHookPayload = 4 << 20
)

// clineHookSpec descreve um hook agent-sync executado em um evento Cline.
//
// Um spec executa OU um `script` (arquivo em <baseDir>/hooks, rodado via
// `bash <script>` com o payload no stdin) OU um `command` (linha shell rodada
// via `bash -c`, usada pelos hooks que nas outras CLIs já são comandos diretos,
// como `ctx-window handoff <cli>`). `script` tem precedência se ambos vierem.
type clineHookSpec struct {
	event   string
	script  string
	command string
	name    string // nome canônico do hook agent-sync (usado só em log)
}

// clineHookSpecs é a tabela de hooks executados pela ponte.
//
// v1 (A-80.1): hooks que só leem `session_id` / `tool_name` / `tool_input` /
// `tool_response`.
//
// v2 (A-80.4): hooks que o texto do comentário de v1 dizia dependerem de
// transcript, mas que na prática toleram o payload normalizado do Cline:
//   - ctx-window summarize-at-stop: usa contador local + idade do summary.md
//     (o transcript é opcional no script);
//   - agent-task-record: `cli=cline` via env e `session_id`; `model` fica
//     `unknown` e tokens ficam nulos (o Cline não expõe transcript) — a
//     telemetria em `.agent-sync/agent_tasks.jsonl` ainda é gravada;
//   - ctx-handoff: `ctx-window handoff cline` (o default do CLI já emite
//     `hookSpecificOutput.additionalContext`).
//
// Continuam de fora (gaps aceitos, ver docs/ADR-cline-hooks-mcp-wiramento.md):
//   - false-success-guard: só age com `transcript_path`, ausente no runtime de
//     plugin do Cline (vira no-op `{}` se wirado);
//   - precompact-snapshot: o runtime de plugin do Cline não expõe PreCompact
//     (mesmo gap aceito registrado para o Cursor).
var clineHookSpecs = []clineHookSpec{
	// PreToolUse (equivalente direto do PreToolUse de Claude/Codex)
	{event: "PreToolUse", script: "bash-rm-guardian.pretooluse.sh", name: bashRmGuardianHookName},
	{event: "PreToolUse", script: "context-guard-nudge.pretooluse.sh", name: contextGuardHookName},
	{event: "PreToolUse", script: "memory-nudge.pretooluse.sh", name: memoryNudgeHookName},
	{event: "PreToolUse", script: "agent-react-nudge.pretooluse.sh", name: agentReactNudgeHookName},
	{event: "PreToolUse", script: "principles-inject.pretooluse.sh", name: principlesInjectHookName},
	{event: "PreToolUse", script: "secret-guard.pretooluse.sh", name: secretGuardPreToolUseHookName},

	// PostToolUse
	{event: "PostToolUse", script: "docs-cache.sh", name: docsCacheHookName},
	{event: "PostToolUse", script: "ctx-window-nudge.sh", name: ctxWindowNudgeHookName},
	{event: "PostToolUse", script: "secret-guard.posttooluse.sh", name: secretGuardPostToolUseHookName},
	{event: "PostToolUse", script: "memory-observe.posttooluse.sh", name: memoryObserveHookName},
	{event: "PostToolUse", script: "token-nudge.check.sh", name: tokenNudgeHookName},

	// TaskStart (proxy de SessionStart)
	{event: "TaskStart", script: "memory-prune-session-start.sh", name: memoryPruneSessionStartHookName},
	{event: "TaskStart", command: "ctx-window handoff cline", name: ctxHandoffHookName},

	// TaskComplete (proxy de Stop)
	{event: "TaskComplete", script: "memory-consolidate.stop.sh", name: memoryConsolidateHookName},
	{event: "TaskComplete", script: "ctx-window-summarize-at-stop.sh", name: ctxWindowSummarizeStopHookName},
	{event: "TaskComplete", script: "agent-task-record.stop.sh", name: agentTaskRecordHookName},
}

// clineBridgeEvents são os eventos Cline para os quais o wiramento cria shim
// (internal/hooks/apply_cline.go). PreCompact fica de fora: o loader de
// arquivos do Cline mapeia o evento para `undefined` e o hook é ignorado.
var clineBridgeEvents = []string{"PreToolUse", "PostToolUse", "TaskStart", "TaskComplete"}

// clinePayload é o subset do payload do Cline que a ponte consome.
type clinePayload struct {
	TaskID         string          `json:"taskId"`
	HookName       string          `json:"hookName"`
	Iteration      int             `json:"iteration"`
	SessionContext clineSessionCtx `json:"sessionContext"`
	WorkspaceRoots []string        `json:"workspaceRoots"`
	UserID         string          `json:"userId"`
	AgentID        string          `json:"agent_id"`
	ToolCall       *clineToolCall  `json:"tool_call"`
	PreToolUse     *clineToolEvent `json:"preToolUse"`
	PostToolUse    *clineToolEvent `json:"postToolUse"`
	UserPromptSub  *clinePrompt    `json:"userPromptSubmit"`
	Prompt         string          `json:"prompt"`
}

type clineSessionCtx struct {
	RootSessionID string `json:"rootSessionId"`
}

type clineToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type clineToolEvent struct {
	ToolName   string          `json:"toolName"`
	Parameters json.RawMessage `json:"parameters"`
	Result     json.RawMessage `json:"result"`
	Success    *bool           `json:"success"`
}

type clinePrompt struct {
	Prompt string `json:"prompt"`
}

// claudeHookPayload é o payload normalizado entregue aos scripts agent-sync
// (mesmo shape que Claude Code / Codex usam hoje).
type claudeHookPayload struct {
	SessionID     string          `json:"session_id"`
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name,omitempty"`
	ToolInput     json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse  json.RawMessage `json:"tool_response,omitempty"`
	Prompt        string          `json:"prompt,omitempty"`
	Cwd           string          `json:"cwd,omitempty"`
	AgentKind     string          `json:"agent_kind"`
}

// claudeHookOutput cobre o contrato de saída Claude/Codex (usado pelos
// scripts agent-sync) + os campos nativos do Cline (para scripts que já
// falem o contrato Cline diretamente).
type claudeHookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		AdditionalContext        string `json:"additionalContext"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
	AdditionalContext   string `json:"additionalContext"`
	SystemMessage       string `json:"systemMessage"`
	Decision            string `json:"decision"`
	Reason              string `json:"reason"`
	Context             string `json:"context"`
	ContextModification string `json:"contextModification"`
	ErrorMessage        string `json:"errorMessage"`
	Cancel              *bool  `json:"cancel"`
	CancelReason        string `json:"cancelReason"`
}

// clineHookResult é o acumulado da execução de todos os scripts de um evento.
type clineHookResult struct {
	contexts []string
	cancel   bool
	reason   string
	warnings []string
}

// clineHookResponse é o JSON final impresso no stdout (contrato Cline).
type clineHookResponse struct {
	Cancel       bool   `json:"cancel"`
	Context      string `json:"context"`
	CancelReason string `json:"cancelReason,omitempty"`
}

// claudeEventForClineEvent traduz o evento Cline para o nome equivalente no
// contrato Claude/Codex — os scripts agent-sync comparam `hook_event_name`.
func claudeEventForClineEvent(event string) string {
	switch normalizeClineEvent(event) {
	case "TaskStart", "TaskResume":
		return "SessionStart"
	case "TaskComplete", "TaskError", "TaskCancel":
		return "Stop"
	case "SessionShutdown":
		return "SessionEnd"
	default:
		return normalizeClineEvent(event)
	}
}

// normalizeClineEvent aceita tanto os nomes de hook dos ARQUIVOS do Cline
// (PreToolUse/PostToolUse/TaskStart/...) quanto os nomes internos usados pelo
// runtime de plugins (tool_call/tool_result/agent_start/...). O shim de
// arquivo e o plugin passam nomes diferentes para o mesmo estágio.
func normalizeClineEvent(event string) string {
	switch event {
	case "tool_call":
		return "PreToolUse"
	case "tool_result":
		return "PostToolUse"
	case "agent_start", "agent_resume":
		return "TaskStart"
	case "agent_end":
		return "TaskComplete"
	case "agent_abort":
		return "TaskCancel"
	case "agent_error":
		return "TaskError"
	case "prompt_submit":
		return "UserPromptSubmit"
	case "session_shutdown":
		return "SessionShutdown"
	default:
		return event
	}
}

// normalizedClinePayload converte o payload Cline (raw) no payload
// Claude-compatível consumido pelos scripts agent-sync. Devolve também a raiz
// do workspace (primeiro workspaceRoot) e o session_id derivado.
func normalizedClinePayload(event string, raw []byte) ([]byte, string, string, error) {
	var p clinePayload
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, "", "", fmt.Errorf("payload Cline invalido: %w", err)
		}
	}

	sessionID := strings.TrimSpace(p.TaskID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(p.SessionContext.RootSessionID)
	}
	if sessionID == "" {
		sessionID = "default"
	}

	var root string
	for _, ws := range p.WorkspaceRoots {
		if strings.TrimSpace(ws) != "" {
			root = strings.TrimSpace(ws)
			break
		}
	}

	norm := claudeHookPayload{
		SessionID:     sessionID,
		HookEventName: claudeEventForClineEvent(event),
		AgentKind:     "cline",
		Cwd:           root,
	}

	switch normalizeClineEvent(event) {
	case "PreToolUse":
		norm.ToolName = toolNameFromEvents(p)
		norm.ToolInput = toolInputFromEvents(p)
	case "PostToolUse":
		norm.ToolName = toolNameFromEvents(p)
		norm.ToolInput = toolInputFromEvents(p)
		if p.PostToolUse != nil && len(p.PostToolUse.Result) > 0 {
			norm.ToolResponse = p.PostToolUse.Result
		}
	case "UserPromptSubmit":
		if p.UserPromptSub != nil && p.UserPromptSub.Prompt != "" {
			norm.Prompt = p.UserPromptSub.Prompt
		} else {
			norm.Prompt = p.Prompt
		}
	}

	data, err := json.Marshal(norm)
	if err != nil {
		return nil, "", "", err
	}
	return data, root, sessionID, nil
}

// toolNameFromEvents extrai o nome da tool do primeiro bloco disponível.
func toolNameFromEvents(p clinePayload) string {
	if p.ToolCall != nil && p.ToolCall.Name != "" {
		return p.ToolCall.Name
	}
	if p.PreToolUse != nil && p.PreToolUse.ToolName != "" {
		return p.PreToolUse.ToolName
	}
	if p.PostToolUse != nil && p.PostToolUse.ToolName != "" {
		return p.PostToolUse.ToolName
	}
	return ""
}

// toolInputFromEvents extrai o input da tool. `tool_call.input` é o objeto
// cru; `preToolUse.parameters`/`postToolUse.parameters` são a serialização em
// string do mesmo objeto (fallback quando o payload vem só do bloco do evento).
func toolInputFromEvents(p clinePayload) json.RawMessage {
	if p.ToolCall != nil && len(p.ToolCall.Input) > 0 && !isJSONNull(p.ToolCall.Input) {
		return normalizeClineToolInput(p.ToolCall.Input)
	}
	for _, ev := range []*clineToolEvent{p.PreToolUse, p.PostToolUse} {
		if ev == nil || len(ev.Parameters) == 0 || isJSONNull(ev.Parameters) {
			continue
		}
		if params := decodeMaybeJSONString(ev.Parameters); params != nil {
			return normalizeClineToolInput(params)
		}
	}
	return nil
}

// normalizeClineToolInput adapta o input da tool do Cline para o formato que os
// scripts agent-sync esperam (contrato Claude/Codex: `tool_input.command`).
// O Cline usa nomes próprios — `run_commands` entrega {"commands": ["a","b"]} —
// então sintetizamos `command` (junção com " && ") quando ele não existe.
func normalizeClineToolInput(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return raw
	}
	var obj map[string]any
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return raw
	}
	if _, hasCommand := obj["command"]; hasCommand {
		return trimmed
	}
	rawCommands, ok := obj["commands"]
	if !ok {
		return trimmed
	}
	list, ok := rawCommands.([]any)
	if !ok {
		return trimmed
	}
	parts := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, strings.TrimSpace(s))
		}
	}
	if len(parts) == 0 {
		return trimmed
	}
	obj["command"] = strings.Join(parts, " && ")
	normalized, err := json.Marshal(obj)
	if err != nil {
		return trimmed
	}
	return normalized
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

// decodeMaybeJSONString devolve o objeto JSON contido em raw, aceitando tanto
// um objeto quanto uma string que contenha um objeto serializado.
func decodeMaybeJSONString(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return trimmed
	}
	if trimmed[0] != '"' {
		return nil
	}
	var inner string
	if err := json.Unmarshal(trimmed, &inner); err != nil {
		return nil
	}
	inner = strings.TrimSpace(inner)
	if inner == "" || !json.Valid([]byte(inner)) {
		return nil
	}
	return json.RawMessage(inner)
}

// translateClaudeHookOutput converte o stdout de UM script agent-sync em
// (contexto, cancel, motivo). `denyMode` decide o que fazer com
// `permissionDecision=deny`/`decision=block`: "stop" => cancel (aborta o run,
// fail-safe), qualquer outro valor => apenas contexto (warn).
func translateClaudeHookOutput(out []byte, denyMode string) (string, bool, string) {
	parsed, ok := parseHookJSON(out)
	if !ok {
		return "", false, ""
	}

	var h claudeHookOutput
	if err := json.Unmarshal(parsed, &h); err != nil {
		return "", false, ""
	}

	ctxParts := []string{}
	for _, v := range []string{
		h.HookSpecificOutput.AdditionalContext,
		h.AdditionalContext,
		h.SystemMessage,
		h.Context,
		h.ContextModification,
		// Cline trata errorMessage como contexto quando cancel=false.
		h.ErrorMessage,
	} {
		if v = strings.TrimSpace(v); v != "" {
			ctxParts = append(ctxParts, v)
		}
	}

	// Bloqueio explícito no contrato Cline (cancel:true) sempre vence.
	if h.Cancel != nil && *h.Cancel {
		reason := strings.TrimSpace(h.CancelReason)
		if reason == "" {
			reason = strings.TrimSpace(h.ErrorMessage)
		}
		return joinContexts(ctxParts), true, reason
	}

	decision := strings.ToLower(strings.TrimSpace(h.HookSpecificOutput.PermissionDecision))
	if decision == "" {
		decision = strings.ToLower(strings.TrimSpace(h.Decision))
	}
	if decision != "deny" && decision != "block" {
		return joinContexts(ctxParts), false, ""
	}

	reason := strings.TrimSpace(h.HookSpecificOutput.PermissionDecisionReason)
	if reason == "" {
		reason = strings.TrimSpace(h.Reason)
	}
	if reason == "" {
		reason = "hook agent-sync negou a acao"
	}
	if denyMode == "stop" {
		return joinContexts(ctxParts), true, reason
	}
	return joinContexts(append(ctxParts, "[blocked by hook] "+reason)), false, ""
}

// parseHookJSON aceita stdout JSON puro ou a última linha JSON de um stdout
// com ruído (defesa para scripts que imprimem avisos antes do JSON).
func parseHookJSON(out []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, false
	}
	if json.Valid(trimmed) {
		return trimmed, true
	}
	lines := bytes.Split(trimmed, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		candidate := bytes.TrimSpace(lines[i])
		if len(candidate) == 0 || candidate[0] != '{' {
			continue
		}
		if json.Valid(candidate) {
			return candidate, true
		}
	}
	return nil, false
}

// joinContexts remove vazios/duplicados e junta com newline.
func joinContexts(parts []string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, "\n")
}

// mergeClineHookResults acumula os resultados dos scripts de um evento.
func mergeClineHookResults(results []clineHookResult) clineHookResult {
	merged := clineHookResult{contexts: []string{}, warnings: []string{}}
	reasons := []string{}
	for _, r := range results {
		merged.contexts = append(merged.contexts, r.contexts...)
		merged.warnings = append(merged.warnings, r.warnings...)
		if r.cancel {
			merged.cancel = true
			if r.reason != "" {
				reasons = append(reasons, r.reason)
			}
		}
	}
	merged.reason = joinContexts(reasons)
	return merged
}

// renderClineHookResponse serializa a resposta no contrato Cline. Sempre
// imprime exatamente um objeto JSON — o Cline faz JSON.parse do stdout inteiro.
func renderClineHookResponse(res clineHookResult) []byte {
	ctx := joinContexts(res.contexts)
	if len(ctx) > maxClineHookContext {
		ctx = ctx[:maxClineHookContext] + "\n[hook context truncated]"
	}
	payload := clineHookResponse{Cancel: res.cancel, Context: ctx, CancelReason: res.reason}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"cancel":false,"context":""}`)
	}
	return data
}

// runClineHookScripts executa os hooks do evento (scripts e/ou comandos) com o
// payload normalizado, coletando contexto/cancel. Falha de um hook nunca
// interrompe os demais nem o run: vira warning em stderr.
func runClineHookScripts(baseDir, event string, payload []byte, root, denyMode string, timeout time.Duration, stderr io.Writer) clineHookResult {
	results := []clineHookResult{}
	for _, spec := range clineHookSpecs {
		if spec.event != normalizeClineEvent(event) {
			continue
		}
		script := ""
		if spec.script != "" {
			script = filepath.Join(baseDir, "hooks", spec.script)
			if _, err := os.Stat(script); err != nil {
				fmt.Fprintf(stderr, "cline-bridge: script ausente (%s), pulando %s\n", script, spec.name)
				continue
			}
		}
		out, err := runClineHookScript(script, spec.command, payload, root, timeout)
		if err != nil {
			results = append(results, clineHookResult{warnings: []string{fmt.Sprintf("%s: %v", spec.name, err)}})
			fmt.Fprintf(stderr, "cline-bridge: %s falhou: %v\n", spec.name, err)
			continue
		}
		ctx, cancel, reason := translateClaudeHookOutput(out, denyMode)
		results = append(results, clineHookResult{
			contexts: []string{ctx},
			cancel:   cancel,
			reason:   reason,
		})
	}
	return mergeClineHookResults(results)
}

// runClineHookScript roda um hook agent-sync com o payload no stdin. Quando
// `command` vem preenchido roda `bash -c <command>`; senão roda o arquivo
// `script` via `bash <script>`. O ambiente marca a CLI de origem (cline) e a
// raiz do projeto (AGENT_SYNC_ROOT), usada por scripts como bash-rm-guardian.
func runClineHookScript(script, command string, payload []byte, root string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var argv []string
	if strings.TrimSpace(command) != "" && strings.TrimSpace(script) == "" {
		argv = []string{"-c", command}
	} else {
		argv = []string{script}
	}
	cmd := exec.CommandContext(ctx, "bash", argv...)
	cmd.Stdin = bytes.NewReader(payload)
	env := append([]string{}, os.Environ()...)
	env = append(env, "AGENT_SYNC_AGENT_KIND=cline", "AGENT_SYNC_CLI=cline")
	if strings.TrimSpace(root) != "" {
		env = append(env, "AGENT_SYNC_ROOT="+root)
	}
	cmd.Env = withPrependedPath(env, clineHookBinDir())

	var stdout, stderrBuf bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("timeout de %s", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(stderrBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("exit %v: %s", err, msg)
	}
	return stdout.Bytes(), nil
}

// clineHookBinDir devolve o diretório do próprio binário agent-sync (onde
// ficam ctx-window/memory-mcp/etc.). O PATH do processo do Cline não inclui
// ~/.local/bin de forma confiável (ver gotcha do ADR), e os hooks de
// TaskStart/TaskComplete dependem de resolver `ctx-window` e `agent-sync`.
func clineHookBinDir() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ""
	}
	dir := filepath.Dir(exe)
	if dir == "" || dir == "." {
		return ""
	}
	return dir
}

// withPrependedPath prepende `dir` ao PATH de `env` (substituindo a entrada
// PATH existente) para que os hooks resolvam binários agent-sync mesmo quando
// o PATH herdado do Cline não os inclui.
func withPrependedPath(env []string, dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return env
	}
	sep := string(os.PathListSeparator)
	out := make([]string, 0, len(env)+1)
	done := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") && !done {
			done = true
			kv = "PATH=" + dir + sep + strings.TrimPrefix(kv, "PATH=")
		}
		out = append(out, kv)
	}
	if !done {
		out = append(out, "PATH="+dir+sep+os.Getenv("PATH"))
	}
	return out
}

// RunClineBridge implementa `agent-sync hook cline --event=<EventName>
// [--base-dir=<path>] [--deny-mode=stop|warn] [--timeout=10s]`.
//
// Sempre imprime uma resposta JSON válida no stdout (mesmo em erro): o Cline
// rejeita o run inteiro se o stdout do hook não for JSON parseável.
func RunClineBridge(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("agent-sync hook cline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	event := fs.String("event", "", "Evento Cline (PreToolUse, PostToolUse, TaskStart, TaskComplete)")
	baseDir := fs.String("base-dir", "", "Raiz do repo agent-sync (default: AGENT_SYNC_HOME ou cwd)")
	denyMode := fs.String("deny-mode", "stop", "Acao em permissionDecision=deny: stop (aborta run) | warn (so contexto)")
	timeout := fs.Duration("timeout", defaultClineHookTimeout, "Timeout por script agent-sync")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*event) == "" {
		return fmt.Errorf("hook cline: --event e obrigatorio")
	}

	raw, err := io.ReadAll(io.LimitReader(stdin, maxClineHookPayload))
	if err != nil {
		fmt.Fprintf(stderr, "cline-bridge: ler stdin: %v\n", err)
		raw = nil
	}

	payload, root, _, err := normalizedClinePayload(*event, raw)
	if err != nil {
		// Payload invalido nao pode derrubar o run: responde no-op.
		fmt.Fprintf(stderr, "cline-bridge: %v\n", err)
		if _, werr := stdout.Write(renderClineHookResponse(clineHookResult{contexts: []string{}})); werr != nil {
			return werr
		}
		return nil
	}

	dir := strings.TrimSpace(*baseDir)
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv("AGENT_SYNC_HOME"))
	}
	if dir == "" {
		if cwd, cerr := os.Getwd(); cerr == nil {
			dir = cwd
		}
	}

	res := runClineHookScripts(dir, *event, payload, root, *denyMode, *timeout, stderr)
	if _, werr := stdout.Write(renderClineHookResponse(res)); werr != nil {
		return werr
	}
	return nil
}
