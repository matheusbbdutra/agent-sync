package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matheusdutra/token-tools/internal/repomap"
)

const (
	mcpProtocolVersion = "2024-11-05"
	mcpServerName      = "agent-sync-code-graph"
	mcpServerVersion   = "1.0.0"
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
	return []map[string]any{
		{
			"name":        "get_file_impact",
			"description": "Retorna o impacto estrutural (blast radius: low|medium|high, acoplamento, tabelas de banco, variáveis de ambiente e comando de teste cirúrgico) de um arquivo no repositório.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":       map[string]any{"type": "string", "description": "Caminho relativo do arquivo no repositório"},
					"max_tokens": map[string]any{"type": "integer", "description": "Limite máximo aproximado de tokens (opcional)"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "get_symbol_callers",
			"description": "Busca todos os arquivos que chamam ou importam um símbolo especificado no repositório.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbol": map[string]any{"type": "string", "description": "Nome do símbolo a buscar (ex.: 'Brief', 'NewStore', 'Save')"},
				},
				"required": []string{"symbol"},
			},
		},
		{
			"name":        "repo_summary",
			"description": "Retorna os símbolos e módulos mais centrais do repositório (top hubs de chamadas e imports).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"max_tokens": map[string]any{"type": "integer", "description": "Limite aproximado de tokens (opcional)"},
				},
			},
		},
	}
}

func textResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

func errorResult(msg string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": "erro: " + msg},
		},
		"isError": true,
	}
}

func callTool(cacheDir, root string, name string, args json.RawMessage) map[string]any {
	cache, err := repomap.Load(cacheDir)
	if err != nil || cache == nil {
		// Auto-update inicial se cache ausente
		var updateErr error
		cache, _, updateErr = repomap.Update(root, cacheDir)
		if updateErr != nil {
			return errorResult(fmt.Sprintf("cache ausente e falha no update: %v", updateErr))
		}
	}

	switch name {
	case "get_file_impact":
		var in struct {
			Path      string `json:"path"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Path) == "" {
			return errorResult("parâmetro 'path' é obrigatório")
		}
		out := repomap.Brief(cache, in.Path, in.MaxTokens)
		return textResult(out)

	case "get_symbol_callers":
		var in struct {
			Symbol string `json:"symbol"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Symbol) == "" {
			return errorResult("parâmetro 'symbol' é obrigatório")
		}
		callers := repomap.FindSymbolCallers(cache, in.Symbol)
		data, err := json.MarshalIndent(map[string]any{
			"symbol":  in.Symbol,
			"callers": callers,
			"count":   len(callers),
		}, "", "  ")
		if err != nil {
			return errorResult(err.Error())
		}
		return textResult(string(data))

	case "repo_summary":
		var in struct {
			MaxTokens int `json:"max_tokens"`
		}
		_ = json.Unmarshal(args, &in)
		out := repomap.Summary(cache, in.MaxTokens)
		return textResult(out)

	default:
		return errorResult("ferramenta desconhecida: " + name)
	}
}

func handle(req rpcRequest, cacheDir, root string) (rpcResponse, bool) {
	switch req.Method {
	case "initialize":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": mcpServerName, "version": mcpServerVersion},
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
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: callTool(cacheDir, root, params.Name, params.Arguments)}, true

	default:
		if len(req.ID) == 0 {
			return rpcResponse{}, false
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "método não encontrado: " + req.Method}}, true
	}
}

func runMCP(cfg *config) int {
	fmt.Fprintf(os.Stderr, "repo-map mcp: servindo cache em %s\n", cfg.cacheDir)

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
			fmt.Fprintf(os.Stderr, "repo-map mcp: mensagem inválida: %v\n", err)
			continue
		}

		resp, ok := handle(req, cfg.cacheDir, cfg.root)
		if !ok {
			continue
		}
		payload, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "repo-map mcp: erro ao serializar resposta: %v\n", err)
			continue
		}
		writer.Write(payload)
		writer.WriteByte('\n')
		writer.Flush()
	}
	return 0
}
