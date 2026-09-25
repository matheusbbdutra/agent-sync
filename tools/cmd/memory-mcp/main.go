// memory-mcp expõe uma memória compartilhada (~/.cache/agent-sync/memory.db,
// libSQL local) como servidor MCP (stdio), permitindo que Claude Code, Codex,
// Antigravity (agy), OpenCode e Cursor leiam e gravem no mesmo histórico de decisões.
//
// Busca hoje é FTS5/BM25 — sem embedding real (ver internal/agentmemory).
package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	agentEnum := []string{"claude-code", "codex", "antigravity", "opencode", "cursor", "user", "tool", "agent-sync"}
	typeEnum := []string{"user", "feedback", "project", "reference", "event"}
	return []map[string]any{
		{
			"name":        "store_memory",
			"description": "Grava ou atualiza uma memória por projeto+type+name, com PC e caminho de origem. Se o content contiver números não-triviais (ex.: '4 commits', '150 linhas'), o campo 'evidence' (output literal de comando) passa a ser OBRIGATÓRIO — gate de validação documentado em preferences/regra-2-pass-write (ses_f2ad21a27ffeasIkrIiVwQc53m, 2026-09-24).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent":        map[string]any{"type": "string", "enum": agentEnum, "description": "Quem está gravando"},
					"session_id":   map[string]any{"type": "string", "description": "ID da sessão de origem (opcional)"},
					"type":         map[string]any{"type": "string", "enum": typeEnum},
					"name":         map[string]any{"type": "string", "description": "slug curto, único por projeto+type"},
					"description":  map[string]any{"type": "string"},
					"content":      map[string]any{"type": "string"},
					"evidence":     map[string]any{"type": "string", "description": "Output literal do comando que produziu o número (git log, git diff --stat, search_memory). OBRIGATÓRIO quando content contiver padrão '\\d+\\s+(commits?|memorias?|linhas|LOC|files?|bytes|tasks?|itens)' e o número não for placeholder (~, N)"},
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
			// A-54 (S-0.2, ses_f2b12742): alias de search_memory com nome
			// alinhado ao ai-memory memory_query (akitaonrails). Mesmo
			// schema; delega para runSearchMemory. Ranking BM25 + decaimento
			// exponencial sobre AccessedAt ja existe em Store.Search
			// (tools/internal/agentmemory/store.go:434).
			"name":        "memory_query",
			"description": "Alias de search_memory — busca por relevância (BM25 + recência) em memórias compartilhadas por texto. Origem: ai-memory memory_query (akitaonrails). Sem entity index nem vector (escopo separado).",
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
		{
			"name":        "record_event",
			"description": "Registra um evento estruturado (decisão, hipótese validada, tarefa concluída/delegada, nudge de guardrail) emitido por hooks/skills. Eventos são memórias com type='event' e ficam na tabela memories — reaproveitam FTS5 e sync Turso. Por padrão são scratch=true (removíveis). Use store_memory se quiser ancorar a decisão como memória permanente.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent":        map[string]any{"type": "string", "enum": agentEnum, "description": "Quem está emitindo"},
					"kind":         map[string]any{"type": "string", "enum": []string{"decision", "hypothesis_validated", "task_completed", "task_delegated", "guard_nudge", "action", "blocker", "open_question", "state_render", "note"}, "description": "Categoria do evento"},
					"note":         map[string]any{"type": "string", "description": "Descrição curta do evento"},
					"source":       map[string]any{"type": "string", "enum": []string{"auto-hook", "manual", "agent"}, "description": "Origem da gravação (default: auto-hook)"},
					"retention":    map[string]any{"type": "string", "enum": []string{"scratch", "permanent"}, "description": "Ciclo de vida (scratch=7d removível, permanent=sem prune)"},
					"session_id":   map[string]any{"type": "string", "description": "ID da sessão de origem (opcional)"},
					"project_path": map[string]any{"type": "string", "description": "Diretório do projeto; se omitido, usa o projeto Git do diretório inicial do MCP"},
					"global":       map[string]any{"type": "boolean", "description": "Vincular a um projeto (default) ou gravar sem projeto (global)"},
					"scratch":      map[string]any{"type": "boolean", "description": "true (default) = removível depois; false = permanente"},
				},
				"required": []string{"agent", "kind", "note"},
			},
		},
		{
			"name":        "list_events",
			"description": "Lista eventos (memórias type='event') do mais recente para o mais antigo. Útil para auditar o que cada hook/skill emitiu durante uma sessão.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind":         map[string]any{"type": "string", "enum": []string{"decision", "hypothesis_validated", "task_completed", "task_delegated", "guard_nudge", "action", "blocker", "open_question", "state_render", "note"}, "description": "Filtrar por categoria (opcional)"},
					"since":        map[string]any{"type": "string", "description": "ISO/RFC3339 — só eventos a partir desse instante (opcional)"},
					"limit":        map[string]any{"type": "integer", "description": "Máximo de resultados (default 100)"},
					"project_id":   map[string]any{"type": "string", "description": "Filtrar pelo ID comum do projeto"},
					"pc":           map[string]any{"type": "string", "description": "Filtrar pelo PC de origem"},
					"project_path": map[string]any{"type": "string", "description": "Filtrar pelo caminho de origem exato"},
					"project_dir":  map[string]any{"type": "string", "description": "Diretório local cujo ID comum será usado para filtrar nos dois PCs"},
				},
			},
		},
		{
			"name":        "memory_read_session",
			"description": "Lista eventos filtrados por session_id (obrigatório). Shim fino sobre ListEventsBySession — Origem: ai-memory memory_read_session_observations (akitaonrails). Complementa list_events com filtro explícito por sessão.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id":   map[string]any{"type": "string", "description": "ID da sessão (obrigatório)"},
					"kind":         map[string]any{"type": "string", "enum": []string{"decision", "hypothesis_validated", "task_completed", "task_delegated", "guard_nudge", "action", "blocker", "open_question", "state_render", "note"}, "description": "Filtrar por categoria (opcional)"},
					"since":        map[string]any{"type": "string", "description": "ISO/RFC3339 — só eventos a partir desse instante (opcional)"},
					"limit":        map[string]any{"type": "integer", "description": "Máximo de resultados (default 100)"},
					"project_id":   map[string]any{"type": "string", "description": "Filtrar pelo ID comum do projeto"},
					"pc":           map[string]any{"type": "string", "description": "Filtrar pelo PC de origem"},
					"project_path": map[string]any{"type": "string", "description": "Filtrar pelo caminho de origem exato"},
					"project_dir":  map[string]any{"type": "string", "description": "Diretório local cujo ID comum será usado para filtrar nos dois PCs"},
				},
				"required": []string{"session_id"},
			},
		},
		{
			"name":        "memory_delete_page",
			"description": "Remove uma memória por path (slug), com semântica de 'página': idempotente (não-erro se não existir) e admitida apenas para scratch=true. Memórias permanentes (scratch=false) são recusadas com mensagem clara. Shins fino sobre delete_memory — Origem: ai-memory memory_delete_page (akitaonrails).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":       map[string]any{"type": "string", "description": "Path/slug da página (ex.: 'docs/adr/ADR-001'). Vira o campo 'name' da memória após normalização."},
					"project_id": map[string]any{"type": "string", "description": "ID do projeto (opcional; default = escopo atual)"},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "memory_read_page",
			"description": "Lê uma memória por path (slug), com semântica de 'página': devolve path + description + content separados. Shim fino sobre get_memory — Origem: ai-memory memory_read_page (akitaonrails). Sem parsing de frontmatter YAML (camada A-56-rabbit se necessário).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":       map[string]any{"type": "string", "description": "Path/slug da página (ex.: 'docs/adr/ADR-001'). Vira o campo 'name' da memória após normalização."},
					"project_id": map[string]any{"type": "string", "description": "ID do projeto (opcional; default = escopo atual)"},
				},
				"required": []string{"path"},
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

// formatMemoryPage é o shim de A-56 para memory_read_page: devolve slug +
// description + content em formato legível, sem parser de frontmatter.
// Imprime o slug (name normalizado) e nao o path literal — consistencia com
// A-60 (memory_delete_page) que ja normaliza antes de delegar.
func formatMemoryPage(slug string, m *agentmemory.Memory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "page: %s\n", slug)
	fmt.Fprintf(&b, "type: %s\n", m.Type)
	fmt.Fprintf(&b, "description: %s\n", m.Description)
	fmt.Fprintf(&b, "agent: %s\n", m.Agent)
	fmt.Fprintf(&b, "updated_at: %s\n", m.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "scratch: %v\n", m.Scratch)
	fmt.Fprintf(&b, "----\n%s\n", m.Content)
	return strings.TrimRight(b.String(), "\n")
}

// runSearchMemory e o nucleo compartilhado por 'search_memory' (tool
// canonica) e 'memory_query' (alias A-54). Faz unmarshal, valida query
// obrigatoria, aplica ScopeFilter e delega para Store.Search (que ja
// combina BM25 + decaimento exponencial sobre AccessedAt, half-life 30d,
// peso 0.5 — ver tools/internal/agentmemory/store.go:434).
func runSearchMemory(store *agentmemory.Store, args json.RawMessage) map[string]any {
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

// evidenceMatch representa uma ocorrencia de numero nao-trivial no content
// de store_memory. Origem: preference/regra-2-pass-write-ses_f2ad21a27ffeasIkrIiVwQc53m
// (2026-09-24). Validado empiricamente com 20 casos (19/20 corretos; falso
// negativo conhecido: "100 tokens" — "tokens" nao esta na lista por YAGNI).
type evidenceMatch struct {
	Value int
	Unit  string
}

// requireEvidence retorna (match, true) se o content contiver um numero
// nao-trivial sem evidence anexada. Heuristica: cardinal solto + unidade
// contavel comum, sem prefixo ~ nem placeholder N, e nao seguido de pontuacao
// que sugira line ref ("linha N") ou versao ("version N.N").
//
// Casos que NAO disparam: datas ISO, IDs com underscore, RFC3339, placeholders
// (~150, N commits, linha 42, version 2.0).
//
// Quando dispara, o agente TEM que rodar o comando (git log, git diff --stat,
// search_memory, etc) e colar o output no campo 'evidence' do store_memory.
// Alternativa: reformular como "~N unidade" (estimativa nao validada).
func requireEvidence(content, evidence string) (evidenceMatch, bool) {
	if evidence != "" {
		return evidenceMatch{}, false
	}
	// Excluir casos conhecidos como nao-contagem
	// 1. Linha de codigo: "linha N", "linha N do ..."
	if regexp.MustCompile(`(?i)\blinha\s+\d+\b`).MatchString(content) {
		return evidenceMatch{}, false
	}
	// 2. Versao semantica: "version N.N", "vN.N"
	if regexp.MustCompile(`(?i)\b(version|v)\s*\d+\.\d+`).MatchString(content) {
		return evidenceMatch{}, false
	}
	// 3. Data ISO no inicio: "2026-09-24 ..."
	if regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\b`).MatchString(strings.TrimSpace(content)) {
		return evidenceMatch{}, false
	}
	// 4. Placeholder N: "N commits", "N totais" etc no inicio
	if regexp.MustCompile(`^N\s+\w+`).MatchString(strings.TrimSpace(content)) {
		return evidenceMatch{}, false
	}

	// Pattern principal (Go RE2 compativel - SEM lookbehind/lookahead, que RE2 nao suporta).
	// \\b antes do cardinal cobre o caso "letra/underscore antes" (word boundary nao match
	// se ha letra imediatamente antes). Para prefixo ~, fazemos check explicito em match[0].
	re := regexp.MustCompile(`\b(\d+)\s+(commits?|memorias?|linhas?|LOC|files?|bytes?|tasks?|itens?|arquivos?|segundos?|minutos?|horas?|dias?|totais|entradas|cenarios?|ocorrencias?|vezes?)\b`)
	m := re.FindStringIndex(content)
	if m == nil {
		return evidenceMatch{}, false
	}
	matchStart := m[0]
	// Rejeita se o caracter imediatamente antes do match for ~ (estimativa intencional).
	if matchStart > 0 && content[matchStart-1] == '~' {
		return evidenceMatch{}, false
	}
	// Re-captura para extrair valor e unidade.
	sub := re.FindStringSubmatch(content)
	if sub == nil {
		return evidenceMatch{}, false
	}
	var val int
	fmt.Sscanf(sub[1], "%d", &val)
	return evidenceMatch{Value: val, Unit: sub[2]}, true
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
			Evidence    string `json:"evidence"`
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
		// Gate de validação (preference/regra-2-pass-write-ses_f2ad21a27ffeasIkrIiVwQc53m,
		// 2026-09-24): se o content contiver número não-trivial solto (ex.: "4 commits",
		// "150 linhas") sem prefixo ~ nem placeholder N, evidence vira OBRIGATÓRIO.
		// Memoria tem que trazer output literal do comando que produziu o numero.
		if missing, ok := requireEvidence(in.Content, in.Evidence); ok {
			return errorResult(fmt.Sprintf(
				"content contém número não-trivial sem evidência anexada: detectei %d %s. "+
					"Rode o comando que produziu o número (git log --since=, git diff --stat, "+
					"search_memory, etc) e cole o output no campo 'evidence'. "+
					"Ou reformule como '~%d %s' (estimativa não validada) ou use placeholder 'N %s'.",
				missing.Value, missing.Unit, missing.Value, missing.Unit, missing.Unit))
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

		// Hook anti-duplicação (Parte 2): checa ANTES do Upsert se já existe memória
		// com mesmo (project_id, type, name). Se sim, anexa aviso ao resultado.
		// NAO recusa — agente decide se deleta antes de regravar ou aceita duplicação.
		// Causa raiz do crescimento exponencial: gravar nova em vez de atualizar.
		var dupWarning string
		if existing, getErr := store.GetScoped(in.Name, origin.ProjectID); getErr == nil && existing != nil {
			dupWarning = fmt.Sprintf(
				"\n⚠ AVISO: já existia memória com name=%q type=%q project_id=%q (updated_at=%s). "+
					"Considere chamar delete_memory antes de regravar para evitar duplicação. "+
					"Se a gravação é atualização consciente, ignore este aviso.",
				existing.Name, existing.Type, existing.ProjectID, existing.UpdatedAt.Format(time.RFC3339))
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
		return textResult(fmt.Sprintf("memória %q gravada (%s)%s%s", in.Name, in.Type, scratchNote, dupWarning))

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
		return runSearchMemory(store, args)

	case "memory_query":
		// A-54 (S-0.2, ses_f2b12742): alias de search_memory com nome alinhado
		// ao ai-memory (akitaonrails). Shim fino — delega para runSearchMemory
		// sem duplicar logica. Ranking BM25+recencia ja existe em store.Search
		// (tools/internal/agentmemory/store.go:434) — nada de RRF novo aqui.
		return runSearchMemory(store, args)

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

	case "record_event":
		var in struct {
			Agent       string `json:"agent"`
			Kind        string `json:"kind"`
			Note        string `json:"note"`
			Source      string `json:"source"`
			Retention   string `json:"retention"`
			SessionID   string `json:"session_id"`
			ProjectPath string `json:"project_path"`
			Global      bool   `json:"global"`
			// *bool distingue "campo omitido" (default true = scratch) de
			// "cliente enviou explicitamente false" (permanente).
			Scratch *bool `json:"scratch"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return errorResult("params inválidos: " + err.Error())
		}
		if in.Agent == "" || in.Kind == "" || strings.TrimSpace(in.Note) == "" {
			return errorResult("campos obrigatórios: agent, kind, note")
		}
		if !agentmemory.EventKind[in.Kind] {
			return errorResult(fmt.Sprintf("kind %q não está no catálogo aceito (%s)", in.Kind, "decision, hypothesis_validated, task_completed, task_delegated, guard_nudge, action, blocker, open_question, state_render, note"))
		}

		source := in.Source
		if source == "" {
			source = "auto-hook"
		}
		if source != "auto-hook" && source != "manual" && source != "agent" {
			return errorResult(fmt.Sprintf("source %q inválido (esperado: auto-hook, manual, agent)", source))
		}

		// ADR §2 Regra de Ouro:
		// auto-hook grava APENAS kind ∈ {action, guard_nudge, state_render} com retention=scratch.
		// Garantia de integridade: se auto-hook tentar gravar decision, hypothesis_validated ou
		// task_completed, é silenciosamente descartado para proteger permanent contra lixo.
		if source == "auto-hook" {
			if in.Kind == "decision" || in.Kind == "hypothesis_validated" || in.Kind == "task_completed" {
				return textResult("evento descartado pela regra de integridade (auto-hook não pode gravar kinds permanentes)")
			}
		}

		// Retention: scratch (default para auto-hook ou quando omitido) vs permanent.
		// Regra de Ouro: auto-hook NUNCA grava permanent (forçado para scratch).
		scratchFlag := true
		if in.Retention == "permanent" {
			scratchFlag = false
		} else if in.Retention == "scratch" {
			scratchFlag = true
		} else if in.Retention != "" {
			return errorResult(fmt.Sprintf("retention %q inválido (esperado: scratch, permanent)", in.Retention))
		} else if in.Scratch != nil {
			// Backward-compat: se retention não foi enviado, respeita scratch booleano legado.
			scratchFlag = *in.Scratch
		}
		if source == "auto-hook" {
			scratchFlag = true // forçado pela regra de ouro
		}

		origin := agentmemory.Origin{}
		if in.Global {
			origin.PC, _ = os.Hostname()
		} else {
			var originErr error
			origin, originErr = agentmemory.ResolveOrigin(in.ProjectPath)
			if originErr != nil {
				return errorResult(originErr.Error())
			}
		}
		// Nome único por timestamp + prefixo de kind + curto hash da nota.
		// Mantém ordenação temporal no índice (ORDER BY updated_at DESC) e
		// permite prefix search em list_events(kind=...).
		short := fmt.Sprintf("%x", sha1.Sum([]byte(in.Note)))[:8]
		evName := fmt.Sprintf("%s-%s-%s", in.Kind, time.Now().UTC().Format("20060102T150405Z"), short)
		desc := in.Note
		if len(desc) > 120 {
			desc = desc[:120]
		}
		err := store.Upsert(agentmemory.Memory{
			Agent: in.Agent, SessionID: in.SessionID, Type: "event",
			Name: evName, Description: desc, Content: in.Note,
			Scratch: scratchFlag,
			PC: origin.PC, ProjectPath: origin.ProjectPath, ProjectID: origin.ProjectID,
		})
		if err != nil {
			return errorResult(err.Error())
		}
		scratchNote := " (permanente)"
		if scratchFlag {
			scratchNote = " (scratch: removível)"
		}
		return textResult(fmt.Sprintf("evento %q registrado (%s)%s", evName, in.Kind, scratchNote))

	case "list_events":
		var in struct {
			Kind        string `json:"kind"`
			Since       string `json:"since"`
			Limit       int    `json:"limit"`
			ProjectID   string `json:"project_id"`
			PC          string `json:"pc"`
			ProjectPath string `json:"project_path"`
			ProjectDir  string `json:"project_dir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return errorResult("params inválidos: " + err.Error())
		}
		if in.Kind != "" && !agentmemory.EventKind[in.Kind] {
			return errorResult(fmt.Sprintf("kind %q não está no catálogo aceito", in.Kind))
		}
		if in.Since != "" {
			if _, err := time.Parse(time.RFC3339, in.Since); err != nil {
				return errorResult("since deve estar em RFC3339 (ex.: 2026-09-20T15:00:00Z)")
			}
		}
		filter, err := memoryFilter(in.ProjectID, in.PC, in.ProjectPath, in.ProjectDir)
		if err != nil {
			return errorResult(err.Error())
		}
		items, err := store.ListEvents(in.Kind, in.Since, in.Limit, filter)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(items) == 0 {
			return textResult("Nenhum evento gravado ainda.")
		}
		return textResult(formatMemories(items))

	case "memory_read_session":
		var in struct {
			SessionID   string `json:"session_id"`
			Kind        string `json:"kind"`
			Since       string `json:"since"`
			Limit       int    `json:"limit"`
			ProjectID   string `json:"project_id"`
			PC          string `json:"pc"`
			ProjectPath string `json:"project_path"`
			ProjectDir  string `json:"project_dir"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.SessionID) == "" {
			return errorResult("parâmetro 'session_id' é obrigatório")
		}
		if in.Kind != "" && !agentmemory.EventKind[in.Kind] {
			return errorResult(fmt.Sprintf("kind %q não está no catálogo aceito", in.Kind))
		}
		if in.Since != "" {
			if _, err := time.Parse(time.RFC3339, in.Since); err != nil {
				return errorResult("since deve estar em RFC3339 (ex.: 2026-09-20T15:00:00Z)")
			}
		}
		filter, err := memoryFilter(in.ProjectID, in.PC, in.ProjectPath, in.ProjectDir)
		if err != nil {
			return errorResult(err.Error())
		}
		items, err := store.ListEventsBySession(in.SessionID, in.Kind, in.Since, in.Limit, filter)
		if err != nil {
			return errorResult(err.Error())
		}
		if len(items) == 0 {
			return textResult(fmt.Sprintf("Nenhum evento para session_id %q.", in.SessionID))
		}
		return textResult(formatMemories(items))

	case "memory_delete_page":
		var in struct {
			Path      string `json:"path"`
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Path) == "" {
			return errorResult("parâmetro 'path' é obrigatório")
		}
		// Normalização: page paths viram name. Strip leading "./" e trailing "/".
		name := strings.Trim(in.Path, "/")
		name = strings.TrimPrefix(name, "./")
		var deleted bool
		var err error
		if in.ProjectID == "" {
			deleted, err = store.Delete(name)
		} else {
			deleted, err = store.DeleteScoped(name, in.ProjectID)
		}
		if err != nil {
			if err == agentmemory.ErrNotScratch {
				return errorResult(fmt.Sprintf("page %q não é scratch (gravada como permanente) — remoção recusada. Se realmente precisa remover, isso exige ação manual deliberada, não via ferramenta.", in.Path))
			}
			return errorResult(err.Error())
		}
		if !deleted {
			return textResult(fmt.Sprintf("Nenhuma page com path %q.", in.Path))
		}
		return textResult(fmt.Sprintf("page %q removida", in.Path))

	case "memory_read_page":
		var in struct {
			Path      string `json:"path"`
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Path) == "" {
			return errorResult("parâmetro 'path' é obrigatório")
		}
		name := strings.Trim(in.Path, "/")
		name = strings.TrimPrefix(name, "./")
		var m *agentmemory.Memory
		var err error
		if in.ProjectID == "" {
			m, err = store.Get(name)
		} else {
			m, err = store.GetScoped(name, in.ProjectID)
		}
		if err != nil {
			return errorResult(err.Error())
		}
		if m == nil {
			return textResult(fmt.Sprintf("Nenhuma page com path %q.", in.Path))
		}
		// Shim leve: devolve 'description' (lido pela get_memory genérica) + 'content'.
		// Sem parser YAML — AGENTS.md §3 pragmático. Se precisarmos de frontmatter
		// estruturado (ex.: multi-line key), vira A-56-rabbit com yaml.v3.
		return textResult(formatMemoryPage(name, m))

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

// BufferObservation representa um evento ou aprendizado efêmero coletado no postToolUse.
type BufferObservation struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Tool      string    `json:"tool"`
	Status    string    `json:"status"`
	Note      string    `json:"note"`
	Source    string    `json:"source,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Retention string    `json:"retention,omitempty"`
	Path      string    `json:"path,omitempty"`
}

func bufferFilePath(sessionID string) string {
	if sessionID == "" {
		sessionID = "default"
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, sessionID)
	dir := filepath.Join(os.TempDir(), "agent-sync-memory-buffer")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, safe+".jsonl")
}

func runBufferRecord(args []string) error {
	fs := flag.NewFlagSet("buffer-record", flag.ContinueOnError)
	session := fs.String("session", "default", "ID da sessão")
	tool := fs.String("tool", "", "ferramenta executada")
	status := fs.String("status", "ok", "status ou exit code")
	note := fs.String("note", "", "resumo ou observação")
	path := fs.String("path", "", "caminho de arquivo ou projeto")
	source := fs.String("source", "auto-hook", "origem da gravação")
	kind := fs.String("kind", "action", "categoria semântica (default action)")
	retention := fs.String("retention", "scratch", "ciclo de vida: scratch | permanent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *note == "" && *tool == "" {
		return fmt.Errorf("campos mínimos ausentes: -tool ou -note")
	}
	obs := BufferObservation{
		Timestamp: time.Now().UTC(),
		SessionID: *session,
		Tool:      *tool,
		Status:    *status,
		Note:      *note,
		Source:    *source,
		Kind:      *kind,
		Retention: *retention,
		Path:      *path,
	}
	bPath := bufferFilePath(*session)
	f, err := os.OpenFile(bPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(obs)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

func runQuery(store *agentmemory.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	query := fs.String("query", "", "texto para busca semântica/FTS5")
	path := fs.String("path", "", "caminho de arquivo para busca de contexto")
	limit := fs.Int("limit", 5, "número máximo de resultados")
	format := fs.String("format", "text", "formato de saída: text | json")
	agent := fs.String("agent", "", "filtrar por agente")
	typ := fs.String("type", "", "filtrar por tipo")
	if err := fs.Parse(args); err != nil {
		return err
	}
	q := strings.TrimSpace(*query)
	if q == "" && *path != "" {
		q = filepath.Base(*path)
	}
	if q == "" {
		return fmt.Errorf("parâmetro -query ou -path é obrigatório")
	}
	filter := agentmemory.ScopeFilter{}
	if *path != "" {
		if origin, err := agentmemory.ResolveOrigin(*path); err == nil {
			filter.ProjectID = origin.ProjectID
			filter.ProjectPath = origin.ProjectPath
		}
	}
	items, err := store.Search(q, *agent, *typ, *limit, filter)
	if err != nil {
		return err
	}
	if *format == "json" {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintf(out, "Nenhuma memória para %q.\n", q)
		return nil
	}
	fmt.Fprintln(out, formatMemories(items))
	return nil
}

func runConsolidate(store *agentmemory.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("consolidate", flag.ContinueOnError)
	session := fs.String("session", "default", "ID da sessão")
	projectPath := fs.String("project", "", "caminho do projeto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	bPath := bufferFilePath(*session)
	f, err := os.Open(bPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "Buffer vazio ou ausente para esta sessão.")
			return nil
		}
		return err
	}
	defer f.Close()

	var observations []BufferObservation
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var o BufferObservation
		if err := json.Unmarshal([]byte(text), &o); err == nil {
			observations = append(observations, o)
		}
	}

	if len(observations) == 0 {
		_ = os.Remove(bPath)
		fmt.Fprintln(out, "Nenhuma observação no buffer.")
		return nil
	}

	origin := agentmemory.Origin{}
	if *projectPath != "" {
		origin, _ = agentmemory.ResolveOrigin(*projectPath)
	} else {
		origin, _ = agentmemory.ResolveOrigin("")
	}

	consolidatedCount := 0
	seen := make(map[string]bool)

	for _, obs := range observations {
		if strings.TrimSpace(obs.Note) == "" {
			continue
		}
		dedupKey := fmt.Sprintf("%s:%s", obs.Tool, obs.Note)
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		kind := obs.Kind
		if kind == "" {
			kind = "action"
		}
		source := obs.Source
		if source == "" {
			source = "auto-hook"
		}
		// Regra de ouro §2: se auto-hook tentar gravar kind permanente, descarta
		if source == "auto-hook" && (kind == "decision" || kind == "hypothesis_validated" || kind == "task_completed") {
			continue
		}
		scratchFlag := true
		if obs.Retention == "permanent" && source != "auto-hook" {
			scratchFlag = false
		}

		sum := sha1.Sum([]byte(obs.Note))
		toolPart := obs.Tool
		if toolPart == "" {
			toolPart = "tool"
		}
		slug := fmt.Sprintf("obs-%s-%s-%x", kind, toolPart, sum[:6])
		slug = strings.ReplaceAll(slug, " ", "-")

		mem := agentmemory.Memory{
			Agent:       "agent-sync",
			SessionID:   obs.SessionID,
			Type:        "project",
			Name:        slug,
			Description: fmt.Sprintf("[%s/%s/%s] %s", kind, obs.Tool, obs.Status, obs.Note),
			Content:     obs.Note,
			PC:          origin.PC,
			ProjectPath: origin.ProjectPath,
			ProjectID:   origin.ProjectID,
			Scratch:     scratchFlag,
		}
		if err := store.Upsert(mem); err == nil {
			consolidatedCount++
		}
	}

	f.Close()
	_ = os.Remove(bPath)

	fmt.Fprintf(out, "%d observação(ões) consolidada(s) com sucesso para a sessão %s.\n", consolidatedCount, *session)
	return nil
}

// runPrune executa a poda de memórias scratch expiradas e sai — não entra no
// loop stdio do servidor MCP. É operação de manutenção local, não uma tool
// MCP: o agente não deve poder disparar remoção em massa via protocolo.
//
// A-67: dry-run default ON (seguro), requer --confirm para realmente deletar.
// Default older-than reduzido de 30d para 7d (alinhado com A-66 snapshot —
// ciclo de prune mais frequente casa com cadência do memory-observe).
//
// Regra de segurança: --confirm=false (default) -> SEMPRE preview, nunca
// deleta. --confirm=true -> deleta (e mostra quantos foram removidos).
// --dry-run é mantido por back-compat/intuição mas é redundante: sem
// --confirm, nada é removido independente do valor de --dry-run.
func runPrune(store *agentmemory.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	olderThan := fs.Duration("older-than", 7*24*time.Hour, "idade mínima (accessed_at/updated_at) para remover memórias scratch")
	dryRun := fs.Bool("dry-run", true, "cosmético: sem --confirm nada é removido de qualquer jeito")
	confirm := fs.Bool("confirm", false, "confirma a remoção (sem --confirm só mostra preview)")
	limit := fs.Int("limit", 10, "número máximo de entradas a listar no preview")
	asJSON := fs.Bool("json", false, "saída em JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	preview, err := store.PreviewScratchOlderThan(*olderThan, *limit)
	if err != nil {
		return fmt.Errorf("prune preview: %w", err)
	}

	if !*confirm {
		// Modo seguro: só mostra preview, não deleta.
		if *asJSON {
			data, _ := json.MarshalIndent(preview, "", "  ")
			fmt.Fprintln(out, string(data))
			return nil
		}
		fmt.Fprintf(out, "memory-mcp prune (dry-run): %d memória(s) scratch mais antiga(s) que %s seriam removidas\n", preview.Total, *olderThan)
		if preview.Total == 0 {
			return nil
		}
		fmt.Fprintf(out, "  mostro apenas as %d mais antigas:\n", len(preview.Entries))
		for _, e := range preview.Entries {
			fmt.Fprintf(out, "    [%s/%s] %s — updated_at=%s\n", e.Type, e.Agent, e.Name, e.UpdatedAt)
		}
		fmt.Fprintln(out, "  Re-rode com --confirm para remover de fato.")
		return nil
	}

	// Confirmado: deleta de verdade.
	n, err := store.PruneScratch(*olderThan)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}
	if *asJSON {
		data, _ := json.MarshalIndent(map[string]any{
			"removed": n, "older_than": olderThan.String(),
		}, "", "  ")
		fmt.Fprintln(out, string(data))
		return nil
	}
	fmt.Fprintf(out, "memory-mcp prune: %d memória(s) scratch removida(s) (mais antigas que %s)\n", n, *olderThan)
	_ = dryRun
	return nil
}

// runStats imprime (ou devolve JSON com) agregacoes uteis para telemetria.
// A-67: nova ferramenta, base para validar impacto de A-66 e monitorar
// crescimento futuro do memory-mcp.
func runStats(store *agentmemory.Store, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "saída em JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := store.Stats()
	if err != nil {
		return fmt.Errorf("stats: %w", err)
	}
	if *asJSON {
		data, _ := json.MarshalIndent(st, "", "  ")
		fmt.Fprintln(out, string(data))
		return nil
	}
	fmt.Fprintln(out, "memory-mcp stats:")
	fmt.Fprintf(out, "  total:     %d\n", st.Total)
	fmt.Fprintf(out, "  scratch:   %d\n", st.ByScratch["scratch"])
	fmt.Fprintf(out, "  permanent: %d\n", st.ByScratch["permanent"])
	if st.Total > 0 {
		fmt.Fprintln(out, "  by_type:")
		for k, v := range st.ByType {
			fmt.Fprintf(out, "    %-12s %d\n", k+":", v)
		}
		fmt.Fprintln(out, "  by_agent:")
		for k, v := range st.ByAgent {
			fmt.Fprintf(out, "    %-16s %d\n", k+":", v)
		}
		if len(st.ByKindScratch) > 0 {
			fmt.Fprintln(out, "  by_kind_scratch:")
			for k, v := range st.ByKindScratch {
				fmt.Fprintf(out, "    %-20s %d\n", k+":", v)
			}
		}
		if st.OldestUpdateAt != "" {
			fmt.Fprintf(out, "  oldest:    %s\n", st.OldestUpdateAt)
		}
		if st.NewestUpdateAt != "" {
			fmt.Fprintf(out, "  newest:    %s\n", st.NewestUpdateAt)
		}
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "buffer-record" {
		if err := runBufferRecord(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "memory-mcp: buffer-record: %v\n", err)
			os.Exit(1)
		}
		return
	}

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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "prune":
			if err := runPrune(store, os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "memory-mcp: prune: %v\n", err)
				os.Exit(1)
			}
			return
		case "query":
			if err := runQuery(store, os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "memory-mcp: query: %v\n", err)
				os.Exit(1)
			}
			return
		case "consolidate":
			if err := runConsolidate(store, os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "memory-mcp: consolidate: %v\n", err)
				os.Exit(1)
			}
			return
		case "stats":
			if err := runStats(store, os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintf(os.Stderr, "memory-mcp: stats: %v\n", err)
				os.Exit(1)
			}
			return
		case "help", "-h", "--help":
			fmt.Println("Uso: memory-mcp [prune | query | buffer-record | consolidate | stats] [flags]")
			return
		}
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
