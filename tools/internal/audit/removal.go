// Package audit implementa auditoria textual de remoção para o repo-map.
//
// O subcommand `repo-map audit removal <target>` varre o repositório em busca
// de referências textuais ao alvo (diretório ou arquivo), classifica cada
// referência em uma das 9 classes suportadas e devolve um ledger JSON
// inspirado no cartographer audit_removal. Diferente do cartographer, este
// pacote é puro Go (single-binary) e usa apenas walk+regex+libsql local —
// sem Bun, sem tiktoken, sem grafo SQLite pré-construído.
//
// Três operações expostas:
//
//	Walk           — varre o repositório (git ls-files ou fallback .gitignore)
//	                e devolve paths relativos, na mesma convenção de
//	                tools/internal/repomap.
//	LiteralHits    — para um conjunto de paths e um alvo, retorna todos os
//	                matches textuais (path, linha, texto da linha, trecho).
//	BuildRemovalAudit — agrega hits e classifica nas 9 classes suportadas.
//
// Este arquivo implementa apenas o esqueleto de walk + matchers (passos 1-2
// do ADR A-73). Classificador 9 classes e libsql introspection vivem em
// arquivos separados do mesmo pacote.
package audit

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DefaultMaxFileBytes limita o tamanho de arquivo lido por hit para evitar
// carregar binários ou logs gigantes em memória.
const DefaultMaxFileBytes = 2 << 20 // 2 MiB

// AuditTarget representa o alvo da auditoria e os matchers regex derivados.
type AuditTarget struct {
	Raw      string          // string original informada pelo usuário
	Matchers []TargetMatcher // matchers compilados (1-3 normalmente)
}

// TargetMatcher é um par (label visível, regex compilada) usado para varrer
// cada linha dos arquivos.
type TargetMatcher struct {
	Label string
	Regex *regexp.Regexp
}

// FileHit é um único match textual encontrado em um arquivo.
type FileHit struct {
	Path     string // path relativo ao root
	Line     int    // 1-indexed
	LineText string
	Match    string // trecho casado (truncado se >120 chars)
}

// NewTarget deriva os matchers a partir da string alvo.
//
// Três matchers canônicos (espelhando cartographer audit.ts:329-344):
//  1. literal exato (case-insensitive)
//  2. subpath @<alvo>/<segmento> (cobre imports JS/TS e Go path imports)
//  3. prefixo UPPER_SNAKE (cobre env vars e nomes derivados)
func NewTarget(raw string) (AuditTarget, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return AuditTarget{}, errors.New("audit: target vazio")
	}
	upper := upperSnake(normalized)
	matchers := []TargetMatcher{
		{Label: normalized, Regex: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(normalized))},
		{Label: "@" + normalized + "/*", Regex: regexp.MustCompile(`@` + regexp.QuoteMeta(normalized) + `/[A-Za-z0-9_.\-]+`)},
	}
	if upper != "" && upper != normalized {
		matchers = append(matchers, TargetMatcher{
			Label: upper + "_*",
			Regex: regexp.MustCompile(`\b` + regexp.QuoteMeta(upper) + `_[A-Z0-9_]+\b`),
		})
	}
	return AuditTarget{Raw: normalized, Matchers: matchers}, nil
}

// upperSnake converte "tools/cmd/memory-mcp" -> "TOOLS_CMD_MEMORY_MCP".
func upperSnake(s string) string {
	var b strings.Builder
	prevLower := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
			prevLower = true
		case r >= 'A' && r <= 'Z':
			if prevLower {
				b.WriteRune('_')
			}
			b.WriteRune(r)
			prevLower = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			prevLower = false
		default:
			// separadores (/, -, .) viram _
			b.WriteRune('_')
			prevLower = false
		}
	}
	out := strings.Trim(b.String(), "_")
	// colapsa múltiplos _ consecutivos
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	return out
}

// Walk devolve os paths relativos do repositório, respeitando .gitignore
// (via `git ls-files -co --exclude-standard`) ou com fallback manual se não
// houver .git. Diretórios comuns (node_modules, vendor, .venv, __pycache__)
// são sempre ignorados.
func Walk(root string) ([]string, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		out, err := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard").Output()
		if err == nil {
			lines := splitNonEmpty(string(out))
			sort.Strings(lines)
			return lines, nil
		}
	}
	return walkFallback(root)
}

// walkFallback atravessa o diretório manualmente com SkipDir nos paths
// ruidosos. Não tenta emular gitignore completo — é o último recurso.
func walkFallback(root string) ([]string, error) {
	skip := map[string]bool{
		".git": true, ".agent-sync": true, "node_modules": true,
		"vendor": true, ".venv": true, "__pycache__": true,
		".cartographer": true, "dist": true, "build": true,
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if skip[filepath.Base(rel)] {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// splitNonEmpty devolve as linhas não-vazias de s.
func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// LiteralHits varre cada path e devolve os matches textuais para os matchers
// do alvo. Arquivos maiores que maxBytes são truncados na leitura; arquivos
// inacessíveis são pulados silenciosamente (best-effort).
func LiteralHits(root string, paths []string, target AuditTarget, maxBytes int) ([]FileHit, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxFileBytes
	}
	seen := make(map[string]struct{}, 1024)
	var hits []FileHit
	for _, rel := range paths {
		abs := filepath.Join(root, rel)
		hits = appendFileHits(hits, seen, abs, rel, target, maxBytes)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Path != hits[j].Path {
			return hits[i].Path < hits[j].Path
		}
		return hits[i].Line < hits[j].Line
	})
	return hits, nil
}

// appendFileHits lê um arquivo e adiciona matches ao slice. Retorna hits
// possivelmente estendido. Erros de I/O são engolidos (best-effort).
func appendFileHits(hits []FileHit, seen map[string]struct{}, abs, rel string, target AuditTarget, maxBytes int) []FileHit {
	f, err := os.Open(abs)
	if err != nil {
		return hits
	}
	defer f.Close()

	// Lê no máximo maxBytes; se o arquivo for maior, ainda assim processa
	// o que couber (a parte lida é onde matches costumam estar).
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	bytesRead := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		bytesRead += len(line) + 1
		for _, m := range target.Matchers {
			for _, mk := range m.Regex.FindAllString(line, -1) {
				if mk == "" {
					continue
				}
				key := rel + ":" + itoa(lineNo) + ":" + mk
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				hits = append(hits, FileHit{
					Path:     rel,
					Line:     lineNo,
					LineText: line,
					Match:    redact(mk),
				})
			}
		}
		if bytesRead >= maxBytes {
			break
		}
	}
	return hits
}

// redact limita trechos a 120 chars para não estourar o JSON output.
func redact(s string) string {
	const max = 120
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// itoa converte int para string sem importar strconv no hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TargetBase devolve o nome curto do alvo (último segmento do path) para
// matching em paths de arquivos (ex: "memory-mcp" para "tools/cmd/memory-mcp").
func (t AuditTarget) TargetBase() string {
	if t.Raw == "" {
		return ""
	}
	return filepath.Base(t.Raw)
}

// String implementa fmt.Stringer.
func (t AuditTarget) String() string {
	return fmt.Sprintf("AuditTarget{raw=%q, matchers=%d}", t.Raw, len(t.Matchers))
}
