package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SQLKeywords é o set de palavras reservadas que nunca devem ser tratadas
// como nome de tabela. Espelha tools/internal/repomap/repomap.go:670-678.
var sqlKeywords = map[string]bool{
	"where": true, "select": true, "set": true, "values": true,
	"order": true, "group": true, "having": true, "limit": true,
	"offset": true, "and": true, "or": true, "not": true,
	"in": true, "is": true, "null": true, "default": true,
	"primary": true, "foreign": true, "key": true, "constraint": true,
	"table": true, "index": true, "on": true, "as": true,
	"case": true, "when": true, "then": true, "else": true,
	"end": true, "distinct": true, "left": true, "right": true,
	"inner": true, "outer": true, "cross": true, "with": true,
	"recursive": true, "union": true, "all": true, "any": true,
	"exists": true, "between": true, "like": true, "glob": true,
	"if": true,
}

var (
	reSQLCreate = regexp.MustCompile(`(?i)\bCREATE\s+(?:TEMP\s+|TEMPORARY\s+)?TABLE\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?["'` + "`" + `]?([a-zA-Z0-9_]+)["'` + "`" + `]?`)
	reSQLInsert = regexp.MustCompile(`(?i)\bINSERT\s+(?:OR\s+\w+\s+)?INTO\s+["'` + "`" + `]?([a-zA-Z0-9_]+)["'` + "`" + `]?`)
	reSQLFrom   = regexp.MustCompile(`(?i)\b(?:FROM|JOIN|UPDATE)\s+["'` + "`" + `]?([a-zA-Z0-9_]+)["'` + "`" + `]?`)
)

// ExtractTablesFromDDL varre o conteúdo procurando DDL/DML SQL e devolve
// os nomes de tabela únicos (lower-case, dedup, sorted, sem keywords).
func ExtractTablesFromDDL(content string) []string {
	seen := map[string]struct{}{}
	for _, re := range []*regexp.Regexp{reSQLCreate, reSQLInsert, reSQLFrom} {
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			name := strings.ToLower(strings.TrimSpace(m[1]))
			if name == "" || sqlKeywords[name] || len(name) < 2 {
				continue
			}
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// ExtractTablesFromFile lê um arquivo .go e extrai nomes de tabela do DDL
// inline (ex.: o pacote tools/internal/agentmemory define `CREATE TABLE IF
// NOT EXISTS memories` em string literals).
func ExtractTablesFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ExtractTablesFromDDL(string(data)), nil
}

// ExtractTablesFromFiles aplica ExtractTablesFromFile em vários paths e
// devolve a união deduplicada. Arquivos inacessíveis são pulados.
//
// Se root for não-vazio, os paths são interpretados como relativos a root
// (filepath.Join). Caso contrário, paths são absolutos.
func ExtractTablesFromFiles(root string, paths []string) []string {
	seen := map[string]struct{}{}
	for _, p := range paths {
		full := p
		if root != "" && !filepath.IsAbs(p) {
			full = filepath.Join(root, p)
		}
		tables, err := ExtractTablesFromFile(full)
		if err != nil {
			continue
		}
		for _, t := range tables {
			seen[t] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
