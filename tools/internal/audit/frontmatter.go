package audit

import (
	"bufio"
	"os"
	"strings"
)

// Frontmatter é o subconjunto YAML que o classifier usa para distinguir
// docs-active vs docs-historical. Apenas os campos relevantes são
// parseados — chaves não reconhecidas são ignoradas.
type Frontmatter struct {
	Status     string   // "active", "archived", "deprecated", "historical"
	Tags       []string // tags livres
	ArchivedAt string   // ISO date se marcada como arquivada
	Raw        map[string]string
}

// HasFrontmatter devolve true se o arquivo parece começar com `---`.
func HasFrontmatter(content string) bool {
	return strings.HasPrefix(strings.TrimLeft(content, "\n"), "---")
}

// ParseFrontmatter lê o arquivo e devolve o frontmatter + o body restante.
// Retorna (nil, content) se não houver frontmatter.
//
// Implementação intencionalmente simples: cobre o subconjunto que
// docs/ADR-*.md do agent-sync usa (status, tags, archived_at). Não é
// um parser YAML completo (decisão consciente — ADR §3 anti-overengineering).
func ParseFrontmatter(path string) (*Frontmatter, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if !HasFrontmatter(string(data)) {
		return nil, data, nil
	}
	return parseFrontmatterBytes(data)
}

func parseFrontmatterBytes(data []byte) (*Frontmatter, []byte, error) {
	scanner := bufio.NewScanner(bytesReader(data))
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	if !scanner.Scan() {
		return nil, data, nil
	}
	first := strings.TrimSpace(scanner.Text())
	if first != "---" {
		return nil, data, nil
	}

	fm := &Frontmatter{Raw: map[string]string{}}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		// remove aspas simples/duplas
		val = strings.Trim(val, `"'`)
		// remove comentários trailing
		if idx := strings.Index(val, " #"); idx >= 0 {
			val = strings.TrimSpace(val[:idx])
		}
		fm.Raw[key] = val
		switch strings.ToLower(key) {
		case "status":
			fm.Status = strings.ToLower(val)
		case "tags":
			fm.Tags = splitTags(val)
		case "archived_at", "archived":
			fm.ArchivedAt = val
		}
	}

	// devolve o body (resto do arquivo depois do ---)
	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}
	return fm, []byte(body.String()), nil
}

func splitTags(val string) []string {
	val = strings.Trim(val, "[]")
	var out []string
	for _, p := range strings.Split(val, ",") {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// IsHistorical usa o frontmatter para decidir se um doc é histórico.
// Ordem de precedência:
//  1. status: archived | deprecated | historical → true
//  2. tag "archived" | "deprecated" | "historical" → true
//  3. archived_at não-vazio → true
func (f *Frontmatter) IsHistorical() bool {
	if f == nil {
		return false
	}
	switch f.Status {
	case "archived", "deprecated", "historical":
		return true
	}
	for _, t := range f.Tags {
		switch strings.ToLower(t) {
		case "archived", "deprecated", "historical":
			return true
		}
	}
	return f.ArchivedAt != ""
}
