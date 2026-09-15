// memory-mcp expõe uma memória compartilhada (~/.cache/agent-sync/memory.db,
// libSQL local) como servidor MCP (stdio), permitindo que Claude Code, Codex,
// Antigravity (agy), OpenCode e Cursor leiam e gravem no mesmo histórico de decisões.
//
// Busca hoje é FTS5/BM25 — sem embedding real (ver internal/agentmemory).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "agent-sync-memory"
	serverVersion   = "1.0.0"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func toolDefinitions() []map[string]any {
	agentEnum := []string{"claude-code", "codex", "antigravity", "opencode", "cursor"}
	typeEnum := []string{"user", "feedback", "project", "reference"}
	return []map[string]any{
		{
			"name":        "store_memory",
			"description": "Grava ou atualiza (upsert por type+name) uma memória compartilhada entre agentes/CLIs.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent":       map[string]any{"type": "string", "enum": agentEnum, "description": "Quem está gravando"},
					"session_id":  map[string]any{"type": "string", "description": "ID da sessão de origem (opcional)"},
					"type":        map[string]any{"type": "string", "enum": typeEnum},
					"name":        map[string]any{"type": "string", "description": "slug curto, único por type"},
					"description": map[string]any{"type": "string"},
					"content":     map[string]any{"type": "string"},
					"scratch":     map[string]any{"type": "boolean", "description": "true = memória descartável (teste/rascunho), pode ser removida depois com delete_memory. false (default) = memória permanente, não removível por essa ferramenta."},
				},
				"required": []string{"agent", "type", "name", "description", "content"},
			},
		},
		{
			"name":        "delete_memory",
			"description": "Remove uma memória pelo nome, mas SÓ se ela foi gravada com scratch=true. Memórias permanentes (scratch=false) são recusadas — precisam de remoção manual deliberada.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": map[string]any{"type": "string"}},
				"required":   []string{"name"},
			},
		},
		{
			"name":        "search_memory",
			"description": "Busca por relevância (BM25) em memórias compartilhadas por texto.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"agent": map[string]any{"type": "string", "enum": agentEnum, "description": "Filtrar por quem gravou (opcional)"},
					"type":  map[string]any{"type": "string", "enum": typeEnum, "description": "Filtrar por tipo (opcional)"},
					"limit": map[string]any{"type": "integer", "description": "Máximo de resultados (default 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "get_memory",
			"description": "Busca uma memória pelo nome exato.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": map[string]any{"type": "string"}},
				"required":   []string{"name"},
			},
		},
		{
			"name":        "list_memories",
			"description": "Lista memórias, opcionalmente filtrando por agente e/ou tipo.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent": map[string]any{"type": "string", "enum": agentEnum},
					"type":  map[string]any{"type": "string", "enum": typeEnum},
					"limit": map[string]any{"type": "integer", "description": "Máximo de resultados (default 100)"},
				},
			},
		},
	}
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func errorResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": true,
	}
}

func formatMemories(items []agentmemory.Memory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d memória(s):\n", len(items))
	for _, m := range items {
		scratchTag := ""
		if m.Scratch {
			scratchTag = " [scratch]"
		}
		fmt.Fprintf(&b, "\n[%s/%s]%s %s (gravado por %s em %s)\n%s\n%s\n",
			m.Type, m.Name, scratchTag, m.Description, m.Agent, m.UpdatedAt.Format("2006-01-02"), strings.Repeat("-", 8), m.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

func callTool(store *agentmemory.Store, name string, args json.RawMessage) map[string]any {
	switch name {
	case "store_memory":
		var in struct {
			Agent       string `json:"agent"`
			SessionID   string `json:"session_id"`
			Type        string `json:"type"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
			Scratch     bool   `json:"scratch"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return errorResult("params inválidos: " + err.Error())
		}
		if in.Agent == "" || in.Type == "" || in.Name == "" || in.Content == "" {
			return errorResult("campos obrigatórios: agent, type, name, content")
		}
		err := store.Upsert(agentmemory.Memory{
			Agent: in.Agent, SessionID: in.SessionID, Type: in.Type,
			Name: in.Name, Description: in.Description, Content: in.Content, Scratch: in.Scratch,
		})
		if err != nil {
			return errorResult(err.Error())
		}
		scratchNote := ""
		if in.Scratch {
			scratchNote = " (scratch: removível depois)"
		}
		return textResult(fmt.Sprintf("memória %q gravada (%s)%s", in.Name, in.Type, scratchNote))

	case "delete_memory":
		var in struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			return errorResult("parâmetro 'name' é obrigatório")
		}
		deleted, err := store.Delete(in.Name)
		if err != nil {
			if err == agentmemory.ErrNotScratch {
				return errorResult(fmt.Sprintf("memória %q não é scratch (gravada como permanente) — remoção recusada. Se realmente precisa remover, isso exige ação manual deliberada, não via ferramenta.", in.Name))
			}
			return errorResult(err.Error())
		}
		if !deleted {
			return textResult(fmt.Sprintf("Nenhuma memória com nome %q.", in.Name))
		}
		return textResult(fmt.Sprintf("memória %q removida", in.Name))

	case "search_memory":
		var in struct {
			Query string `json:"query"`
			Agent string `json:"agent"`
			Type  string `json:"type"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Query) == "" {
			return errorResult("parâmetro 'query' é obrigatório")
		}
		items, err := store.Search(in.Query, in.Agent, in.Type, in.Limit)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(items) == 0 {
			return textResult(fmt.Sprintf("Nenhuma memória para %q.", in.Query))
		}
		return textResult(formatMemories(items))

	case "get_memory":
		var in struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			return errorResult("parâmetro 'name' é obrigatório")
		}
		m, err := store.Get(in.Name)
		if err != nil {
			return errorResult(err.Error())
		}
		if m == nil {
			return textResult(fmt.Sprintf("Nenhuma memória com nome %q.", in.Name))
		}
		return textResult(formatMemories([]agentmemory.Memory{*m}))

	case "list_memories":
		var in struct {
			Agent string `json:"agent"`
			Type  string `json:"type"`
			Limit int    `json:"limit"`
		}
		json.Unmarshal(args, &in)
		items, err := store.List(in.Agent, in.Type, in.Limit)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(items) == 0 {
			return textResult("Nenhuma memória gravada ainda.")
		}
		return textResult(formatMemories(items))

	default:
		return errorResult("ferramenta desconhecida: " + name)
	}
}

func handle(req rpcRequest, store *agentmemory.Store) (rpcResponse, bool) {
	switch req.Method {
	case "initialize":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": serverVersion},
		}}, true

	case "notifications/initialized", "initialized":
		return rpcResponse{}, false

	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}, true

	case "tools/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": toolDefinitions()}}, true

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32602, Message: "params inválidos"}}, true
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: callTool(store, params.Name, params.Arguments)}, true

	default:
		if len(req.ID) == 0 {
			return rpcResponse{}, false
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "método não encontrado: " + req.Method}}, true
	}
}

func main() {
	dbPath, err := agentmemory.DefaultDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-mcp: %v\n", err)
		os.Exit(1)
	}
	store, err := agentmemory.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-mcp: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()
	fmt.Fprintf(os.Stderr, "memory-mcp: servindo %s\n", dbPath)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "memory-mcp: mensagem inválida: %v\n", err)
			continue
		}

		resp, ok := handle(req, store)
		if !ok {
			continue
		}
		payload, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "memory-mcp: erro ao serializar resposta: %v\n", err)
			continue
		}
		writer.Write(payload)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
