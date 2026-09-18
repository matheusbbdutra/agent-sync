// memory-mcp expõe uma memória compartilhada (~/.cache/agent-sync/memory.db,
// libSQL local) como servidor MCP (stdio), permitindo que Claude Code, Codex,
// Antigravity (agy), OpenCode e Cursor leiam e gravem no mesmo histórico de decisões.
//
// Busca hoje é FTS5/BM25 — sem embedding real (ver internal/agentmemory).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
			"description": "Grava ou atualiza uma memória por projeto+type+name, com PC e caminho de origem.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent":        map[string]any{"type": "string", "enum": agentEnum, "description": "Quem está gravando"},
					"session_id":   map[string]any{"type": "string", "description": "ID da sessão de origem (opcional)"},
					"type":         map[string]any{"type": "string", "enum": typeEnum},
					"name":         map[string]any{"type": "string", "description": "slug curto, único por projeto+type"},
					"description":  map[string]any{"type": "string"},
					"content":      map[string]any{"type": "string"},
					"project_path": map[string]any{"type": "string", "description": "Diretório do projeto; se omitido, usa o projeto Git do diretório inicial do MCP"},
					"global":       map[string]any{"type": "boolean", "description": "Gravar memória sem vínculo com projeto"},
					"scratch":      map[string]any{"type": "boolean", "description": "true = memória descartável (teste/rascunho), pode ser removida depois com delete_memory. false (default) = memória permanente, não removível por essa ferramenta."},
				},
				"required": []string{"agent", "type", "name", "description", "content"},
			},
		},
		{
			"name":        "delete_memory",
			"description": "Remove uma memória pelo nome, mas SÓ se ela foi gravada com scratch=true. Memórias permanentes (scratch=false) são recusadas — precisam de remoção manual deliberada.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"}},
				"required":   []string{"name"},
			},
		},
		{
			"name":        "search_memory",
			"description": "Busca por relevância (BM25) em memórias compartilhadas por texto.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":        map[string]any{"type": "string"},
					"agent":        map[string]any{"type": "string", "enum": agentEnum, "description": "Filtrar por quem gravou (opcional)"},
					"type":         map[string]any{"type": "string", "enum": typeEnum, "description": "Filtrar por tipo (opcional)"},
					"limit":        map[string]any{"type": "integer", "description": "Máximo de resultados (default 10)"},
					"project_id":   map[string]any{"type": "string", "description": "Filtrar pelo ID comum do projeto"},
					"pc":           map[string]any{"type": "string", "description": "Filtrar pelo PC de origem"},
					"project_path": map[string]any{"type": "string", "description": "Filtrar pelo caminho de origem exato"},
					"project_dir":  map[string]any{"type": "string", "description": "Diretório local cujo ID comum será usado para filtrar nos dois PCs"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "get_memory",
			"description": "Busca uma memória pelo nome exato e, se necessário, pelo projeto.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"name": map[string]any{"type": "string"}, "project_id": map[string]any{"type": "string"}},
				"required":   []string{"name"},
			},
		},
		{
			"name":        "list_memories",
			"description": "Lista memórias, opcionalmente filtrando por agente e/ou tipo.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent":        map[string]any{"type": "string", "enum": agentEnum},
					"type":         map[string]any{"type": "string", "enum": typeEnum},
					"limit":        map[string]any{"type": "integer", "description": "Máximo de resultados (default 100)"},
					"project_id":   map[string]any{"type": "string", "description": "Filtrar pelo ID comum do projeto"},
					"pc":           map[string]any{"type": "string", "description": "Filtrar pelo PC de origem"},
					"project_path": map[string]any{"type": "string", "description": "Filtrar pelo caminho de origem exato"},
					"project_dir":  map[string]any{"type": "string", "description": "Diretório local cujo ID comum será usado para filtrar nos dois PCs"},
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
		fmt.Fprintf(&b, "\n[%s/%s]%s %s (gravado por %s em %s; PC: %s; projeto: %s; caminho: %s)\n%s\n%s\n",
			m.Type, m.Name, scratchTag, m.Description, m.Agent, m.UpdatedAt.Format("2006-01-02"), m.PC, m.ProjectID, m.ProjectPath, strings.Repeat("-", 8), m.Content)
	}
	return strings.TrimRight(b.String(), "\n")
}

func memoryFilter(projectID, pc, projectPath, projectDir string) (agentmemory.ScopeFilter, error) {
	if projectDir != "" {
		origin, err := agentmemory.ResolveOrigin(projectDir)
		if err != nil {
			return agentmemory.ScopeFilter{}, err
		}
		if projectID != "" && projectID != origin.ProjectID {
			return agentmemory.ScopeFilter{}, fmt.Errorf("project_id difere do projeto em project_dir")
		}
		projectID = origin.ProjectID
	}
	return agentmemory.ScopeFilter{ProjectID: projectID, PC: pc, ProjectPath: projectPath}, nil
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
			ProjectPath string `json:"project_path"`
			Global      bool   `json:"global"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return errorResult("params inválidos: " + err.Error())
		}
		if in.Agent == "" || in.Type == "" || in.Name == "" || in.Content == "" {
			return errorResult("campos obrigatórios: agent, type, name, content")
		}
		origin := agentmemory.Origin{}
		if in.Global {
			origin.PC, _ = os.Hostname()
		}
		if !in.Global {
			var originErr error
			origin, originErr = agentmemory.ResolveOrigin(in.ProjectPath)
			if originErr != nil {
				return errorResult(originErr.Error())
			}
		}
		err := store.Upsert(agentmemory.Memory{
			Agent: in.Agent, SessionID: in.SessionID, Type: in.Type,
			Name: in.Name, Description: in.Description, Content: in.Content, Scratch: in.Scratch,
			PC: origin.PC, ProjectPath: origin.ProjectPath, ProjectID: origin.ProjectID,
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
			Name      string `json:"name"`
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			return errorResult("parâmetro 'name' é obrigatório")
		}
		var deleted bool
		var err error
		if in.ProjectID == "" {
			deleted, err = store.Delete(in.Name)
		} else {
			deleted, err = store.DeleteScoped(in.Name, in.ProjectID)
		}
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
			Query       string `json:"query"`
			Agent       string `json:"agent"`
			Type        string `json:"type"`
			Limit       int    `json:"limit"`
			ProjectID   string `json:"project_id"`
			PC          string `json:"pc"`
			ProjectPath string `json:"project_path"`
			ProjectDir  string `json:"project_dir"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Query) == "" {
			return errorResult("parâmetro 'query' é obrigatório")
		}
		filter, err := memoryFilter(in.ProjectID, in.PC, in.ProjectPath, in.ProjectDir)
		if err != nil {
			return errorResult(err.Error())
		}
		items, err := store.Search(in.Query, in.Agent, in.Type, in.Limit, filter)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(items) == 0 {
			return textResult(fmt.Sprintf("Nenhuma memória para %q.", in.Query))
		}
		return textResult(formatMemories(items))

	case "get_memory":
		var in struct {
			Name      string `json:"name"`
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Name) == "" {
			return errorResult("parâmetro 'name' é obrigatório")
		}
		var m *agentmemory.Memory
		var err error
		if in.ProjectID == "" {
			m, err = store.Get(in.Name)
		} else {
			m, err = store.GetScoped(in.Name, in.ProjectID)
		}
		if err != nil {
			return errorResult(err.Error())
		}
		if m == nil {
			return textResult(fmt.Sprintf("Nenhuma memória com nome %q.", in.Name))
		}
		return textResult(formatMemories([]agentmemory.Memory{*m}))

	case "list_memories":
		var in struct {
			Agent       string `json:"agent"`
			Type        string `json:"type"`
			Limit       int    `json:"limit"`
			ProjectID   string `json:"project_id"`
			PC          string `json:"pc"`
			ProjectPath string `json:"project_path"`
			ProjectDir  string `json:"project_dir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return errorResult("params inválidos: " + err.Error())
		}
		filter, err := memoryFilter(in.ProjectID, in.PC, in.ProjectPath, in.ProjectDir)
		if err != nil {
			return errorResult(err.Error())
		}
		items, err := store.List(in.Agent, in.Type, in.Limit, filter)
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

// runPrune executa a poda de memórias scratch expiradas e sai — não entra no
// loop stdio do servidor MCP. É operação de manutenção local, não uma tool
// MCP: o agente não deve poder disparar remoção em massa via protocolo.
func runPrune(store *agentmemory.Store, args []string) {
	fs := flag.NewFlagSet("prune", flag.ExitOnError)
	olderThan := fs.Duration("older-than", 30*24*time.Hour, "idade mínima (accessed_at/updated_at) para remover memórias scratch")
	fs.Parse(args)
	n, err := store.PruneScratch(*olderThan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory-mcp: prune: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("memory-mcp: %d memória(s) scratch removida(s) (mais antigas que %s)\n", n, *olderThan)
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

	if len(os.Args) > 1 && os.Args[1] == "prune" {
		runPrune(store, os.Args[2:])
		return
	}

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
