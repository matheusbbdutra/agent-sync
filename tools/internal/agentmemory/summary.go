// Persistência de resumos do ctx-window na memória compartilhada.
//
// O YAML gerado pelo LLM summarizer tem 6 seções (decisions, active_hypotheses,
// artifacts, resolved_errors, next_steps, constraints). Esta camada persiste
// cada item de cada seção como uma memória individual — assim, quando o agente
// fizer `search_memory("decisão sobre LLM cascade")`, o BM25 casa com a entrada
// específica, não com um blob YAML gigante onde o termo aparece enterrado.
//
// Naming estável por hash do conteúdo: se a mesma decisão reaparece em outra
// compactação, o Upsert sobrescreve (mesma chave) em vez de duplicar. O
// histórico de compactações continua preservado nos `summary_vN.md` da sessão
// — esta tabela é só a projeção consultável.

package agentmemory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// summarySections é a lista de seções que esperamos no YAML produzido pelo
// summarizer. Mantida alinhada com skills/context-window-strategy/prompts/summarize.md
// (decisions, active_hypotheses, artifacts, resolved_errors, next_steps, constraints).
var summarySections = []string{
	"decisions",
	"active_hypotheses",
	"artifacts",
	"resolved_errors",
	"next_steps",
	"constraints",
}

// SummaryRecord é um item individual extraído do YAML — é o que vira uma
// linha na tabela memories.
type SummaryRecord struct {
	Section     string // "decisions", "next_steps", ...
	Key         string // nome estável: section+hash do texto
	Description string // descrição curta para indexação
	Content     string // texto completo do item
}

// ParseSummary extrai itens por seção do YAML produzido pelo summarizer.
// Não é um parser YAML completo — só entende o formato que o próprio
// summarizePromptTemplate (tools/cmd/ctx-window/summarize.go) emite: listas
// top-level `decisions: ["..."]` e mapas `artifacts: [{path: ..., description: ...}]`.
// Formato divergente vira sem erro, apenas com menos itens — é melhor do
// que rejeitar tudo porque o LLM botou uma linha a mais.
func ParseSummary(yaml string) []SummaryRecord {
	var out []SummaryRecord
	for _, section := range summarySections {
		body := extractSection(yaml, section)
		if body == "" {
			continue
		}
		switch section {
		case "artifacts", "resolved_errors":
			for _, line := range splitListItems(body) {
				if k, d, c, ok := parseKeyedEntry(section, line); ok {
					out = append(out, SummaryRecord{Section: section, Key: k, Description: d, Content: c})
				}
			}
		default:
			for _, item := range splitListItems(body) {
				out = append(out, makeRecord(section, item))
			}
		}
	}
	return out
}

// UpsertSummary grava cada item extraído como uma memória individual
// (type="project", name estável por hash) e retorna quantos foram gravados
// ou atualizados. Falha de uma entrada não aborta as demais — cada Upsert
// é independente. projectID identifica o escopo; cliName e sessionID vão
// como metadados para auditoria via list_memories.
func (s *Store) UpsertSummary(projectID, cliName, sessionID, yaml string) (int, error) {
	records := ParseSummary(yaml)
	if len(records) == 0 {
		return 0, nil
	}
	written := 0
	for _, r := range records {
		err := s.Upsert(Memory{
			Agent:       cliName,
			SessionID:   sessionID,
			Type:        "project",
			Name:        "ctx-" + r.Section + "-" + shortHash(r.Content),
			Description: sectionToDescription(r.Section),
			Content:     r.Content,
			ProjectID:   projectID,
		})
		if err != nil {
			return written, fmt.Errorf("agentmemory: gravar item %s/%s: %w", r.Section, r.Key, err)
		}
		written++
	}
	return written, nil
}

func makeRecord(section, text string) SummaryRecord {
	return SummaryRecord{
		Section:     section,
		Key:         section + "-" + shortHash(text),
		Description: sectionToDescription(section),
		Content:     text,
	}
}

func sectionToDescription(section string) string {
	return "ctx-window: " + section
}

// shortHash devolve os primeiros 16 hex chars de sha256 — chave curta o
// suficiente pra caber no nome, ainda única o suficiente pra evitar colisão
// em projetos do tamanho deste repo.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// extractSection devolve o corpo de uma seção top-level `name:` até a próxima
// seção top-level ou fim do YAML. Aceita indentação variável (LLM costuma
// variar entre 0 e 4 espaços). Não é YAML-compliant; é bom o bastante pro
// formato específico que o summarizePromptTemplate pede.
func extractSection(yaml, section string) string {
	lines := strings.Split(yaml, "\n")
	var collected []string
	var inSection bool
	var sectionIndent int
	prefix := section + ":"
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if !inSection {
			stripped := strings.TrimLeft(trimmed, " ")
			if stripped == prefix || strings.HasPrefix(stripped, prefix+" ") || strings.HasPrefix(stripped, prefix+":") {
				inSection = true
				sectionIndent = len(line) - len(stripped)
				// conteúdo inline após "section:": `decisions: ["a", "b"]`
				rest := strings.TrimPrefix(stripped, prefix)
				rest = strings.TrimSpace(rest)
				if rest != "" && rest != "|" && rest != ">" {
					collected = append(collected, expandInline(rest)...)
				}
			}
			continue
		}
		if trimmed == "" || (len(line) > sectionIndent && (line[sectionIndent] == ' ' || line[sectionIndent] == '\t')) {
			collected = append(collected, line)
			continue
		}
		break // próxima seção top-level
	}
	return strings.Join(collected, "\n")
}

// expandInline trata o caso `decisions: ["a", "b"]` em uma única linha,
// comum quando o LLM agrupa a saída para economizar tokens.
func expandInline(s string) []string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
		var out []string
		for _, raw := range strings.Split(inner, ",") {
			item := strings.TrimSpace(raw)
			item = strings.Trim(item, `"`)
			if item != "" {
				out = append(out, "- "+item)
			}
		}
		return out
	}
	return []string{"- " + s}
}

// splitListItems devolve o conteúdo de cada item `- ...` de uma lista YAML.
// Para entradas keyed (artifacts/resolved_errors) — onde o LLM emite
// `- path: "...", description: "..."` em uma linha OU em bloco de várias
// linhas começando com `- path:` — agrupa linhas até o próximo `- ` no
// mesmo nível de indentação. Não lida com listas aninhadas; o summarizer
// não emite.
func splitListItems(body string) []string {
	var out []string
	var current []string
	var currentIndent int
	flush := func() {
		if len(current) == 0 {
			return
		}
		out = append(out, joinItem(current))
		current = nil
	}
	for _, line := range strings.Split(body, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "-") {
			flush()
			// primeiro item após "- ": pode estar inline (`- path: "..."`)
			// ou na linha seguinte (`- path:` na linha de baixo).
			rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(s, "-"), " "))
			currentIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			if rest != "" {
				current = []string{rest}
			}
			continue
		}
		// Continuação do item atual? só se for mais profunda que o "-".
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if len(current) > 0 && lineIndent > currentIndent {
			current = append(current, s)
			continue
		}
		// linha fora do item atual — provavelmente já era hora de parar
		break
	}
	flush()
	return out
}

// joinItem une as linhas de um item em um único "line" para o parser
// keyed. Para itens inline (`- path: "..."`), é a própria string; para
// itens em bloco, junta com vírgula (`path: "...", description: "..."`) para
// que o scanKV continue funcionando.
func joinItem(lines []string) string {
	if len(lines) == 1 {
		return strings.Trim(lines[0], `"`)
	}
	joined := strings.Join(lines, ", ")
	return strings.Trim(joined, `"`)
}

// parseKeyedEntry processa itens do tipo `path: "...", description: "..."` ou
// `cause: "...", fix: "..."` (artifacts e resolved_errors). Aceita uma única
// linha ou YAML block.
func parseKeyedEntry(section, line string) (key, description, content string, ok bool) {
	lower := strings.ToLower(line)
	switch section {
	case "artifacts":
		path := scanKV(line, "path")
		if path == "" {
			return "", "", "", false
		}
		desc := scanKV(line, "description")
		key = "path:" + path + "-" + shortHash(line)
		content = line
		return key, desc, content, true
	case "resolved_errors":
		cause := scanKV(line, "cause")
		fix := scanKV(line, "fix")
		if cause == "" && fix == "" {
			return "", "", "", false
		}
		key = "cause:" + cause + "-" + shortHash(line)
		summary := ""
		if cause != "" {
			summary = cause
		}
		if fix != "" {
			if summary != "" {
				summary += " → " + fix
			} else {
				summary = fix
			}
		}
		return key, summary, line, true
	}
	_ = lower
	return "", "", "", false
}

// scanKV extrai o valor de uma chave `key: value` dentro de uma linha YAML.
// Tenta achar `key: "..."` (com aspas) ou `key: bare`. Tolera vírgulas finais.
func scanKV(line, key string) string {
	idx := strings.Index(line, key+":")
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(key)+1:]
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	if rest[0] == '"' {
		end := strings.Index(rest[1:], `"`)
		if end < 0 {
			return rest[1:]
		}
		return rest[1 : 1+end]
	}
	// bare: corta na vírgula ou no fim
	if comma := strings.Index(rest, ","); comma >= 0 {
		rest = rest[:comma]
	}
	return strings.TrimSpace(rest)
}
