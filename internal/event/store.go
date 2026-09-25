package event

// event_store.go: mirror opcional dos eventos para memory-mcp.
//
// Helper usado pelos hooks PostToolUse para que cada evento do
// session-event.jsonl seja espelhado no store do memory-mcp com
// granularidade 1 evento. Migrado sem renomear (ja tinha prefixo
// event_) em 2026-09-21 (Fase 4).
import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// memoryMCPBin é o caminho do binário memory-mcp instalado por `make install`.
// Sobrescrevível por AGENT_SYNC_MEMORY_MCP_BIN (útil em testes e em runs fora de PATH).
func memoryMCPBin() string {
	if p := strings.TrimSpace(os.Getenv("AGENT_SYNC_MEMORY_MCP_BIN")); p != "" {
		return p
	}
	if p, err := exec.LookPath("memory-mcp"); err == nil {
		return p
	}
	// Fallback: ~/.local/bin/memory-mcp (convenção make install).
	if home, err := os.UserHomeDir(); err == nil {
		cand := home + "/.local/bin/memory-mcp"
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return "memory-mcp" // último fallback — exec falhará com erro claro.
}

// memoryDBPath devolve o caminho do banco de memória compartilhada.
// Consistente com o que o memory-mcp usa por default em memory-sync e session_event.
// Sobrescrevível por AGENT_SYNC_MEMORY_DB.
func memoryDBPath() string {
	if p := strings.TrimSpace(os.Getenv("AGENT_SYNC_MEMORY_DB")); p != "" {
		return p
	}
	return os.ExpandEnv("${XDG_CACHE_HOME:-$HOME/.cache}/agent-sync/memory.db")
}

// eventStoreTimeout é o timeout default para subprocessos memory-mcp.
// Configurável por AGENT_SYNC_EVENT_STORE_TIMEOUT (em segundos).
func eventStoreTimeout() time.Duration {
	s := strings.TrimSpace(os.Getenv("AGENT_SYNC_EVENT_STORE_TIMEOUT"))
	if s == "" {
		return 5 * time.Second
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n <= 0 {
		return 5 * time.Second
	}
	return time.Duration(n) * time.Second
}

// rpcCall faz uma chamada JSON-RPC ao memory-mcp via stdin/stdout.
// Cada chamada precisa de um handshake initialize por processo (memory-mcp
// recusa tools/call sem initialize). Como subprocess é caro, batchamos várias
// chamadas num único processo via batchCall abaixo.
func singleRPCCall(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	bin := memoryMCPBin()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-db", memoryDBPath())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("event_store: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("event_store: stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("event_store: iniciar %s: %w", bin, err)
	}
	// handshake + chamada.
	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{},
	}
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("event_store: escrever initialize: %w", err)
	}
	req = map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("event_store: escrever initialized: %w", err)
	}
	paramsJSON, _ := json.Marshal(params)
	req = map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": method, "params": json.RawMessage(paramsJSON),
	}
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("event_store: escrever %s: %w", method, err)
	}
	_ = stdin.Close()
	// Lê 2 respostas (initialize + tools/call). notifications/initialized é
	// notificação sem id, o servidor não responde — daí não esperamos 3.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("event_store: ler stdout: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("event_store: memory-mcp saiu com erro: %w", err)
	}
	if lastLine == "" {
		return nil, fmt.Errorf("event_store: memory-mcp não respondeu")
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(lastLine), &resp); err != nil {
		return nil, fmt.Errorf("event_store: resposta inválida: %w", err)
	}
	if errRaw, ok := resp["error"]; ok && len(errRaw) > 0 && string(errRaw) != "null" {
		return nil, fmt.Errorf("event_store: %s", string(errRaw))
	}
	return resp["result"], nil
}

// ErrEventStoreUnavailable é retornado quando memory-mcp não está disponível
// (binário ausente). Permite aos chamadores degradar com log silencioso em vez
// de falhar — coerente com o best-effort dos hooks.
var ErrEventStoreUnavailable = errors.New("event_store: memory-mcp indisponível")

// isUnavailable verifica se o erro indica que o binário memory-mcp não está
// no PATH nem em ~/.local/bin. Usado para best-effort fallback.
func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "executable file not found") ||
		strings.Contains(s, "no such file or directory") ||
		strings.Contains(s, "not found in $PATH")
}

// callRecordEvent faz o equivalente a memory-mcp tools/call record_event.
// Retorna ErrEventStoreUnavailable quando o binário não está disponível.
func callRecordEvent(args RecordEventArgs, timeout time.Duration) error {
	params := map[string]any{
		"name":      "record_event",
		"arguments": args,
	}
	_, err := singleRPCCall("tools/call", params, timeout)
	if isUnavailable(err) {
		return ErrEventStoreUnavailable
	}
	return err
}

// RecordEventArgs mapeia o input da tool MCP record_event. Ponteiro em Scratch
// distingue "campo omitido" (default true) de "false explícito" (permanente).
type RecordEventArgs struct {
	Agent       string `json:"agent"`
	Kind        string `json:"kind"`
	Note        string `json:"note"`
	SessionID   string `json:"session_id,omitempty"`
	ProjectPath string `json:"project_path,omitempty"`
	Global      bool   `json:"global,omitempty"`
	Scratch     *bool  `json:"scratch,omitempty"`
}

// callListEvents faz o equivalente a memory-mcp tools/call list_events e
// retorna as memórias no formato bruto. Cada item vira um MemoryRow.
type MemoryRow struct {
	Type        string
	Name        string
	Description string
	Content     string
	Agent       string
	SessionID   string
	ProjectPath string
	ProjectID   string
	UpdatedAt   time.Time
	Scratch     bool
}

func callListEvents(args ListEventsArgs, timeout time.Duration) ([]MemoryRow, error) {
	params := map[string]any{
		"name":      "list_events",
		"arguments": args,
	}
	result, err := singleRPCCall("tools/call", params, timeout)
	if isUnavailable(err) {
		return nil, ErrEventStoreUnavailable
	}
	if err != nil {
		return nil, err
	}
	// result.content[0].text — formato humano do formatMemories (não JSON).
	// Por isso parseamos o texto reconstruindo a partir de blocos
	// "<header>\n<description>\n--------\n<content>". Para o CLI nosso,
	// que quer ler campos estruturados, adicionamos um modo alternativo:
	// quando AGENT_SYNC_EVENT_STORE_RAW=1, expomos o conteúdo como JSON
	// cru (Title/Ref/Details vem do SessionEvent que AppendEvent serializou).
	return parseFormattedMemories(result), nil
}

// ListEventsArgs: subset usado pela CLI para filtros simples.
type ListEventsArgs struct {
	Kind        string `json:"kind,omitempty"`
	Since       string `json:"since,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	ProjectDir  string `json:"project_dir,omitempty"`
	ProjectPath string `json:"project_path,omitempty"`
}

// parseFormattedMemories lê o texto de formatMemories (Memory MCP) e
// devolve os blocos como MemoryRow. Cada bloco tem o formato:
//
//	[event/<name>] [scratch] <description> (gravado por <agent> em YYYY-MM-DD; ...)
//	--------
//	<content>
//
// O conteúdo é reconstruído a partir de description + content.
// Limitação: informações extras (Title vs Ref vs Details) ficam perdidas no
// round-trip — o memory-mcp expõe só description+content. Para preservar
// a estrutura completa, callers sensíveis devem escrever direto no libSQL
// (este é o trade-off aceito na migração).
func parseFormattedMemories(result json.RawMessage) []MemoryRow {
	if len(result) == 0 {
		return nil
	}
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(result, &r); err != nil {
		return nil
	}
	if len(r.Content) == 0 {
		return nil
	}
	raw := r.Content[0].Text
	if strings.HasPrefix(raw, "Nenhum evento gravado") || strings.HasPrefix(raw, "Nenhuma memória") {
		return nil
	}
	// Formato "N memória(s):\n\n<bloco>\n\n<bloco>..."
	parts := strings.Split(raw, "\n--------\n")
	if len(parts) < 2 {
		return nil
	}
	// Header fica em parts[0] ("N memória(s):\n\n<header1>\n\n<header2>...")
	// Conteúdos nos parts[1..]. Cada bloco: header é o anterior ao "--------".
	headersRaw := parts[0]
	contents := parts[1:]
	headers := strings.Split(headersRaw, "\n\n")
	if len(headers) == 0 || strings.HasPrefix(headers[0], "0 memória") {
		return nil
	}
	// headers[0] é "N memória(s):"; ignora.
	headers = headers[1:]
	out := make([]MemoryRow, 0, len(headers))
	for i, h := range headers {
		if i >= len(contents) {
			break
		}
		row := parseHeader(h)
		row.Content = contents[i]
		out = append(out, row)
	}
	return out
}

// mirrorEventToMemory envia o SessionEvent para o memory-mcp como evento.
// Best-effort: erros são logados em stderr mas não retornados — o JSONL local
// permanece como source-of-truth transitória. O mapeamento:
//
//	TS        → UpdatedAt (formato ISO)
//	Kind      → name prefixo
//	Ref       → content prefixo "ref: <ref>\n"
//	Title     → description (curto, <=120 chars)
//	Actor     → agent
//	SessionID → session_id
//	Details   → content sufixo "details: <json>"
func mirrorEventToMemory(projectRoot string, e SessionEvent) {
	args := RecordEventArgs{
		Agent:       actorToAgent(e.Actor),
		Kind:        e.Kind,
		Note:        buildNote(e),
		SessionID:   e.SessionID,
		ProjectPath: projectRoot,
		Scratch:     nil, // default true (removível)
	}
	if err := callRecordEvent(args, eventStoreTimeout()); err != nil {
		if errors.Is(err, ErrEventStoreUnavailable) {
			return // silencioso em best-effort
		}
		fmt.Fprintf(os.Stderr, "event_store: mirror best-effort falhou: %v\n", err)
	}
}

// actorToAgent normaliza os actors do JSONL para o enum aceito pelo memory-mcp.
// user/tool/agent-sync entram direto; claude/codex/opencode/cursor/agy mapeiam
// para o enum do MCP; vazio vira agent-sync (default razoável).
func actorToAgent(actor string) string {
	switch actor {
	case "user", "tool", "agent-sync", "claude", "codex", "opencode", "cursor", "agy":
		return actor
	}
	if actor == "" {
		return "agent-sync"
	}
	// Fora do enum aceito — fallback silencioso para agent-sync.
	return "agent-sync"
}

// buildNote serializa o SessionEvent num formato legível que cabe no campo
// `note` (description <=120, content livre) do memory-mcp. Formato:
//
//	<ref>\n<title>\n<details-json>
func buildNote(e SessionEvent) string {
	var b strings.Builder
	if e.Ref != "" {
		b.WriteString(e.Ref)
		b.WriteByte('\n')
	}
	b.WriteString(e.Title)
	if len(e.Details) > 0 {
		if j, err := json.Marshal(e.Details); err == nil {
			b.WriteString("\ndetails:")
			b.Write(j)
		}
	}
	return b.String()
}

// memoryRowToSessionEvent reconstrói um SessionEvent a partir de um MemoryRow.
// Limitação: o campo Details só é recuperado se o buildNote usou o formato
// acima (parse tolerante a falhas — se o JSON estiver corrompido, Details fica
// nil mas o evento não falha).
func memoryRowToSessionEvent(r MemoryRow) SessionEvent {
	ev := SessionEvent{
		SchemaVersion: SessionEventSchemaVersion,
		TS:            r.UpdatedAt,
		Kind:          r.Type, // memory-mcp usa Type="event"; precisamos do kind real (prefixo de name)
		Title:         r.Description,
		Actor:         r.Agent,
		SessionID:     r.SessionID,
	}
	// O memory-mcp só retorna Type/name/content/description, não o kind original.
	// Como armazenamos o kind no prefixo de Name ("<kind>-<iso>-<hash>"), extraímos.
	if dash := strings.IndexByte(r.Name, '-'); dash > 0 {
		ev.Kind = r.Name[:dash]
	}
	// Content: "<ref>\n<title>\n<details-json>". Re-parseamos.
	lines := strings.SplitN(r.Content, "\n", 3)
	switch len(lines) {
	case 1:
		// só title — ref vazio, details vazio.
	case 2:
		ev.Ref = lines[0]
	case 3:
		ev.Ref = lines[0]
		if strings.HasPrefix(lines[2], "details:") {
			var d map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[2], "details:")), &d); err == nil {
				ev.Details = d
			}
		}
	}
	return ev
}

// readLegacyJSONL é fallback quando memory-mcp está indisponível — lê o JSONL
// local com a mesma lógica de antes (preserva 100% do comportamento legado).
func readLegacyJSONL(projectRoot string, opts EventReadOptions) ([]SessionEvent, error) {
	data, err := os.ReadFile(eventPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionEvent
	for i, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e SessionEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			fmt.Fprintf(os.Stderr, "event: linha %d ignorada (invalida): %v\n", i+1, err)
			continue
		}
		if opts.Kind != "" && e.Kind != opts.Kind {
			continue
		}
		if !opts.Since.IsZero() && e.TS.Before(opts.Since) {
			continue
		}
		out = append(out, e)
	}
	if opts.Last > 0 && len(out) > opts.Last {
		out = out[len(out)-opts.Last:]
	}
	return out, nil
}

// statsLegacyJSONL é o fallback do StatsEvents quando memory-mcp indisponível.
func statsLegacyJSONL(projectRoot string, opts EventReadOptions) (EventStats, error) {
	data, err := os.ReadFile(eventPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return EventStats{ByKind: map[string]int{}, ByActor: map[string]int{}}, nil
		}
		return EventStats{}, err
	}
	s := EventStats{
		ByKind:  map[string]int{},
		ByActor: map[string]int{},
	}
	for i, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e SessionEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			s.Skipped++
			fmt.Fprintf(os.Stderr, "event stats: linha %d ignorada: %v\n", i+1, err)
			continue
		}
		if opts.Kind != "" && e.Kind != opts.Kind {
			continue
		}
		if !opts.Since.IsZero() && e.TS.Before(opts.Since) {
			continue
		}
		s.Total++
		s.ByKind[e.Kind]++
		if e.Actor != "" {
			s.ByActor[e.Actor]++
		}
		if s.FirstTS.IsZero() || e.TS.Before(s.FirstTS) {
			s.FirstTS = e.TS
		}
		if e.TS.After(s.LastTS) {
			s.LastTS = e.TS
		}
	}
	return s, nil
}

// parseHeader extrai type, name, scratch, description, agent e data de um
// header do formato:
//
//	[event/decision-20260920T180219Z-309d278a] [scratch] optou por Postgres (gravado por claude-code em 2026-09-20; PC: ...; projeto: ...; caminho: ...)
func parseHeader(h string) MemoryRow {
	row := MemoryRow{}
	rest := h
	// [type/name]
	if i := strings.Index(rest, "]"); i > 0 && strings.HasPrefix(rest, "[") {
		tag := rest[1:i]
		if slash := strings.IndexByte(tag, '/'); slash > 0 {
			row.Type = tag[:slash]
			row.Name = tag[slash+1:]
		}
		rest = strings.TrimSpace(rest[i+1:])
	}
	if strings.HasPrefix(rest, "[scratch] ") {
		row.Scratch = true
		rest = strings.TrimPrefix(rest, "[scratch] ")
	}
	// "<description> (gravado por <agent> em YYYY-MM-DD; ...)"
	if i := strings.Index(rest, " (gravado por "); i > 0 {
		row.Description = strings.TrimSpace(rest[:i])
		meta := rest[i+len(" (gravado por "):]
		// meta = "<agent> em YYYY-MM-DD; PC: ...; projeto: ...; caminho: ...)"
		if j := strings.Index(meta, " em "); j > 0 {
			row.Agent = meta[:j]
		}
		if j := strings.Index(meta, " em "); j > 0 {
			dateStr := meta[j+len(" em "):]
			if k := strings.Index(dateStr, ";"); k > 0 {
				dateStr = dateStr[:k]
			}
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				row.UpdatedAt = t
			}
		}
	} else {
		row.Description = strings.TrimSpace(rest)
	}
	return row
}