// docs-mcp expõe o cache local de docs e ferramentas de economia de tokens
// (ast-outline, trace-strip) como um servidor MCP (stdio).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/matheusdutra/token-tools/internal/astoutline"
	"github.com/matheusdutra/token-tools/internal/docscache"
	"github.com/matheusdutra/token-tools/internal/tracestrip"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "agent-sync-docs"
	serverVersion   = "1.1.0"
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
			"name":        "search_docs",
			"description": "Busca um termo (case-insensitive) nas documentações já cacheadas localmente e retorna trechos com a URL de origem. Funciona offline.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Termo a buscar (ex.: 'lazy loading', 'autowire')"},
					"limit": map[string]any{"type": "integer", "description": "Máximo de fontes (default 20)"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "list_docs",
			"description": "Lista as documentações disponíveis no cache local.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "ast_outline",
			"description": "Extrai o esqueleto estrutural (classes, métodos, interfaces, funções com números de linha) de um arquivo de código ou snippet, economizando tokens ao inspecionar arquivos grandes. Suporta Go, Python, JS/TS, PHP.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "Caminho do arquivo no sistema de arquivos local."},
					"content": map[string]any{"type": "string", "description": "Conteúdo do código caso não queira ler diretamente do disco (opcional)."},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "strip_trace",
			"description": "Filtra stack traces e logs extensos, ocultando frames internos de runtime/frameworks/vendors (node_modules, vendor/, runtime Go, Spring, etc.) e mantendo apenas as linhas relevantes.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"trace":     map[string]any{"type": "string", "description": "Texto do stack trace ou log a filtrar."},
					"max_lines": map[string]any{"type": "integer", "description": "Máximo de linhas a exibir (default 25)."},
				},
				"required": []string{"trace"},
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

func callTool(cacheDir, name string, args json.RawMessage) map[string]any {
	switch name {
	case "search_docs":
		var in struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Query) == "" {
			return errorResult("parâmetro 'query' é obrigatório")
		}
		matches, err := docscache.Search(cacheDir, in.Query, in.Limit)
		if err != nil {
			return errorResult(fmt.Sprintf("erro na busca: %v", err))
		}
		if len(matches) == 0 {
			return textResult(fmt.Sprintf("Nenhum resultado para %q. Rode 'docs-fetch -update' para sincronizar mais docs.", in.Query))
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d fonte(s) com %q:\n", len(matches), in.Query)
		for _, m := range matches {
			fmt.Fprintf(&b, "\n%s\n", m.URL)
			if m.Heading != "" && m.Heading != "Geral" {
				fmt.Fprintf(&b, "  [%s]\n", m.Heading)
			}
			for _, line := range m.Lines {
				fmt.Fprintf(&b, "  %s\n", line)
			}
		}
		return textResult(strings.TrimRight(b.String(), "\n"))

	case "list_docs":
		entries, err := docscache.List(cacheDir)
		if err != nil {
			return errorResult(fmt.Sprintf("erro ao listar: %v", err))
		}
		if len(entries) == 0 {
			return textResult("Cache vazio. Rode 'docs-fetch -mirror mirror/sources.json'.")
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d documentação(ões) em cache:\n", len(entries))
		for _, e := range entries {
			fmt.Fprintf(&b, "- %s\n", e.URL)
		}
		return textResult(strings.TrimRight(b.String(), "\n"))

	case "ast_outline":
		var in struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Path) == "" {
			return errorResult("parâmetro 'path' é obrigatório")
		}
		var src []byte
		if in.Content != "" {
			src = []byte(in.Content)
		}
		out, err := astoutline.Extract(in.Path, src)
		if err != nil {
			return errorResult(fmt.Sprintf("erro ao extrair outline: %v", err))
		}
		return textResult(out)

	case "strip_trace":
		var in struct {
			Trace    string `json:"trace"`
			MaxLines int    `json:"max_lines"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Trace) == "" {
			return errorResult("parâmetro 'trace' é obrigatório")
		}
		if in.MaxLines <= 0 {
			in.MaxLines = 25
		}
		out := tracestrip.StripText(in.Trace, in.MaxLines)
		return textResult(out)

	default:
		return errorResult("ferramenta desconhecida: " + name)
	}
}

func handle(req rpcRequest, cacheDir string) (rpcResponse, bool) {
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
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: callTool(cacheDir, params.Name, params.Arguments)}, true

	default:
		if len(req.ID) == 0 {
			return rpcResponse{}, false
		}
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: "método não encontrado: " + req.Method}}, true
	}
}

func main() {
	cacheDir := docscache.DefaultDir()
	fmt.Fprintf(os.Stderr, "docs-mcp: servindo %s\n", cacheDir)

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
			fmt.Fprintf(os.Stderr, "docs-mcp: mensagem inválida: %v\n", err)
			continue
		}

		resp, ok := handle(req, cacheDir)
		if !ok {
			continue
		}
		payload, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docs-mcp: erro ao serializar resposta: %v\n", err)
			continue
		}
		writer.Write(payload)
		writer.WriteByte('\n')
		writer.Flush()
	}
}
