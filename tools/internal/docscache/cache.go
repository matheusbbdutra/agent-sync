// Package docscache implementa o cache local de documentação usado por
// docs-fetch e docs-mcp. Permite consulta offline por título/trecho.
package docscache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// DefaultDir devolve o diretório de cache padrão (~/.cache/agent-sync/docs).
func DefaultDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "agent-sync", "docs")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "agent-sync", "docs")
	}
	return "."
}

// Key gera o identificador estável de uma URL.
func Key(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

type Entry struct {
	URL         string
	ContentType string
	Body        string
	Text        string
}

func paths(dir, url string) (bodyPath, metaPath, urlPath, textPath string) {
	base := filepath.Join(dir, Key(url))
	return base + ".body", base + ".body.meta", base + ".url", base + ".txt"
}

// Save grava corpo, content-type, URL e texto extraído no cache.
func Save(dir, url, contentType, body, text string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bodyPath, metaPath, urlPath, textPath := paths(dir, url)
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, []byte(contentType), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(urlPath, []byte(url), 0o644); err != nil {
		return err
	}
	return os.WriteFile(textPath, []byte(text), 0o644)
}

// Load recupera uma entrada do cache pela URL.
func Load(dir, url string) (Entry, bool) {
	bodyPath, metaPath, urlPath, textPath := paths(dir, url)
	body, err := os.ReadFile(bodyPath)
	if err != nil {
		return Entry{}, false
	}
	meta, _ := os.ReadFile(metaPath)
	storedURL, _ := os.ReadFile(urlPath)
	text, _ := os.ReadFile(textPath)
	entry := Entry{ContentType: string(meta), Body: string(body), Text: string(text)}
	entry.URL = string(storedURL)
	if entry.URL == "" {
		entry.URL = url
	}
	return entry, true
}

// List devolve todas as entradas com texto no cache.
func List(dir string) ([]Entry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".url") {
			continue
		}
		url, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(url) == 0 {
			continue
		}
		entry, ok := Load(dir, string(url))
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

// Section representa um bloco coeso de documentação delimitado por cabeçalhos.
type Section struct {
	Heading string
	Anchor  string
	Lines   []string
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeDiacritics(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 'á', 'à', 'ã', 'â', 'ä':
			b.WriteRune('a')
		case 'é', 'è', 'ê', 'ë':
			b.WriteRune('e')
		case 'í', 'ì', 'î', 'ï':
			b.WriteRune('i')
		case 'ó', 'ò', 'õ', 'ô', 'ö':
			b.WriteRune('o')
		case 'ú', 'ù', 'û', 'ü':
			b.WriteRune('u')
		case 'ç':
			b.WriteRune('c')
		case 'ñ':
			b.WriteRune('n')
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// toAnchor converte um título em slug para âncora HTML (ex.: "## Mapeamento Básico" -> "mapeamento-basico").
func toAnchor(heading string) string {
	cleaned := strings.ToLower(strings.TrimLeft(heading, "# "))
	normalized := normalizeDiacritics(cleaned)
	slug := nonAlphaNum.ReplaceAllString(normalized, "-")
	return strings.Trim(slug, "-")
}

// ExtractSections fragmenta um texto Markdown/texto plano em seções baseadas em títulos (#).
func ExtractSections(text string) []Section {
	var sections []Section
	lines := strings.Split(text, "\n")

	current := Section{
		Heading: "Geral",
		Anchor:  "",
		Lines:   nil,
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if len(current.Lines) > 0 {
				sections = append(sections, current)
			}
			current = Section{
				Heading: trimmed,
				Anchor:  toAnchor(trimmed),
				Lines:   nil,
			}
			continue
		}
		if trimmed != "" {
			current.Lines = append(current.Lines, trimmed)
		}
	}

	if len(current.Lines) > 0 {
		sections = append(sections, current)
	}

	return sections
}

// Match representa um resultado de busca contextualizada offline.
type Match struct {
	URL     string
	Heading string
	Anchor  string
	Lines   []string
}

// Search procura um termo (case-insensitive) nas entradas do cache, retornando o bloco de seção.
func Search(dir, term string, limit int) ([]Match, error) {
	entries, err := List(dir)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	needle := strings.ToLower(term)
	var matches []Match

	for _, entry := range entries {
		sections := ExtractSections(entry.Text)
		for _, sec := range sections {
			secMatchesHeading := strings.Contains(strings.ToLower(sec.Heading), needle)
			var matchedLines []string

			for _, line := range sec.Lines {
				if secMatchesHeading || strings.Contains(strings.ToLower(line), needle) {
					if len(line) > 300 {
						line = line[:300] + "…"
					}
					matchedLines = append(matchedLines, line)
					if len(matchedLines) >= 4 {
						break
					}
				}
			}

			if secMatchesHeading || len(matchedLines) > 0 {
				finalURL := entry.URL
				if sec.Anchor != "" && !strings.Contains(finalURL, "#") {
					finalURL = finalURL + "#" + sec.Anchor
				}
				matches = append(matches, Match{
					URL:     finalURL,
					Heading: sec.Heading,
					Anchor:  sec.Anchor,
					Lines:   matchedLines,
				})
				if len(matches) >= limit {
					return matches, nil
				}
			}
		}
	}
	return matches, nil
}
