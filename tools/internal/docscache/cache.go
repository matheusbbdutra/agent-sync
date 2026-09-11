// Package docscache implementa o cache local de documentação usado por
// docs-fetch e docs-mcp. Permite consulta offline por título/trecho.
package docscache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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

// Match representa um resultado de busca offline.
type Match struct {
	URL   string
	Lines []string
}

// Search procura um termo (case-insensitive) no texto das entradas do cache.
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
		var lines []string
		for _, line := range strings.Split(entry.Text, "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				line = strings.TrimSpace(line)
				if len(line) > 300 {
					line = line[:300] + "…"
				}
				lines = append(lines, line)
				if len(lines) >= 5 {
					break
				}
			}
		}
		if len(lines) > 0 {
			matches = append(matches, Match{URL: entry.URL, Lines: lines})
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches, nil
}
