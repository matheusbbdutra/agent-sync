package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArchiveEntry é uma linha do archive/index.jsonl — metadata de 1 tool
// result arquivado. Append-only; jamais modificado após gravação.
type ArchiveEntry struct {
	ID           string    `json:"id"`            // sha256[:8] do conteúdo
	Tool         string    `json:"tool"`          // nome da tool (Read|Bash|...)
	Bytes        int       `json:"bytes"`         // tamanho original
	PreviewChars int       `json:"preview_chars"` // quantos chars ficaram no preview
	At           time.Time `json:"at"`            // quando foi arquivado
	Path         string    `json:"path"`          // path absoluto do .txt no archive
}

const (
	archiveDirName     = "archive"
	archiveIndexName   = "index.jsonl"
	archivePreviewLen  = 500
	archiveDefaultAt   = 4096 // bytes mínimo para arquivar (alinhado com token-optimizer externo)
	archiveSearchLimit = 50   // limite de matches em --search
)

// archiveThreshold devolve o limite de bytes para arquivar (env
// AGENT_SYNC_CTX_ARCHIVE_AT) ou archiveDefaultAt (4KB).
func archiveThreshold() int {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CTX_ARCHIVE_AT")); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return archiveDefaultAt
}

// archiveDir garante que <sessionDir>/archive existe.
func archiveDir(sessionID string) (string, error) {
	dir, err := EnsureSessionDir(sessionID)
	if err != nil {
		return "", err
	}
	ad := filepath.Join(dir, archiveDirName)
	if err := os.MkdirAll(ad, 0o700); err != nil {
		return "", fmt.Errorf("ctx-window: mkdir archive dir: %w", err)
	}
	return ad, nil
}

// archiveIndexPath devolve o path do index.jsonl da sessão.
func archiveIndexPath(sessionID string) (string, error) {
	dir, err := archiveDir(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, archiveIndexName), nil
}

// ArchiveResult grava `content` em <session>/archive/<id>.txt (id
// derivado de sha256[:8]) e appenda metadata no index.jsonl. Retorna
// a ArchiveEntry persistida. Best-effort: se já existir, devolve a entry
// existente sem regravar.
func ArchiveResult(sessionID, tool, content string) (*ArchiveEntry, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("ctx-window: session id vazio")
	}
	if len(content) < archiveThreshold() {
		return nil, nil // abaixo do threshold: nada a fazer
	}
	id := archiveID(content)
	dir, err := archiveDir(sessionID)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".txt")

	// dedup: se já existe, não regrava
	entry := ArchiveEntry{
		ID:           id,
		Tool:         tool,
		Bytes:        len(content),
		PreviewChars: archivePreviewLen,
		At:           time.Now().UTC(),
		Path:         path,
	}
	if _, err := os.Stat(path); err == nil {
		// já existe — append entry só se ainda não estiver no index
		existing, _ := LoadArchiveIndex(sessionID)
		for _, e := range existing {
			if e.ID == id {
				return &e, nil
			}
		}
	} else {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return nil, fmt.Errorf("ctx-window: write archive file: %w", err)
		}
	}

	// append no index
	idxPath, err := archiveIndexPath(sessionID)
	if err != nil {
		return nil, err
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(idxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("ctx-window: open archive index: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("ctx-window: write archive index: %w", err)
	}
	return &entry, nil
}

// archiveID devolve sha256[:8] do content, hex-encoded. Determinístico
// para dedup de re-archives do mesmo conteúdo.
func archiveID(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8])
}

// LoadArchiveIndex lê todas as linhas do index.jsonl. Tolerante a linhas
// corrompidas (skip silencioso). Retorna slice vazia se não existir.
// Lê linha por linha (json.Unmarshal) em vez de json.Decoder, porque
// o Decoder trava quando Decode() falha em linha não-JSON: `More()`
// continua retornando true sem avançar o offset.
func ReadArchiveEntries(path string) ([]ArchiveEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ArchiveEntry
	for _, line := range bytesSplitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e ArchiveEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip linha corrompida
		}
		out = append(out, e)
	}
	return out, nil
}

// LoadArchiveIndex lê todas as linhas do index.jsonl da sessão.
func LoadArchiveIndex(sessionID string) ([]ArchiveEntry, error) {
	idxPath, err := archiveIndexPath(sessionID)
	if err != nil {
		return nil, err
	}
	return ReadArchiveEntries(idxPath)
}

func bytesSplitLines(b []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			lines = append(lines, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		lines = append(lines, b[start:])
	}
	return lines
}

// MaybeArchive é o helper "pure": decide se `content` deve ser arquivado
// e devolve (preview, archiveID, archived). Usado por hook wirado no
// futuro — não modifica estado. Se len < threshold: devolve o conteúdo
// inteiro sem archive.
func MaybeArchive(content string) (preview, id string, archived bool) {
	if len(content) < archiveThreshold() {
		return content, "", false
	}
	id = archiveID(content)
	if len(content) <= archivePreviewLen+128 {
		return content, id, false // pequeno demais para preview valer a pena
	}
	preview = content[:archivePreviewLen] + fmt.Sprintf(
		"\n\n[... archived: %d bytes total; id=%s; run `ctx-window expand <session> %s` to retrieve]",
		len(content), id, id,
	)
	return preview, id, true
}

// ExpandByID lê o conteúdo arquivado pelo id. Retorna erro se não existir.
func ExpandByID(sessionID, id string) (string, *ArchiveEntry, error) {
	idx, err := LoadArchiveIndex(sessionID)
	if err != nil {
		return "", nil, err
	}
	for i := range idx {
		if idx[i].ID == id {
			body, err := os.ReadFile(idx[i].Path)
			if err != nil {
				return "", nil, fmt.Errorf("ctx-window: read archive %s: %w", id, err)
			}
			return string(body), &idx[i], nil
		}
	}
	return "", nil, fmt.Errorf("ctx-window: archive id %q not found in session %q", id, sessionID)
}

// SearchArchive devolve entries cujo conteúdo arquivado contém `query`
// (case-insensitive substring match). Limita a archiveSearchLimit
// matches para evitar I/O explosivo.
func SearchArchive(sessionID, query string) ([]ArchiveEntry, error) {
	if query == "" {
		return nil, errors.New("ctx-window: --search query vazia")
	}
	idx, err := LoadArchiveIndex(sessionID)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var matches []ArchiveEntry
	for _, e := range idx {
		if len(matches) >= archiveSearchLimit {
			break
		}
		body, err := os.ReadFile(e.Path)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(body)), q) {
			matches = append(matches, e)
		}
	}
	return matches, nil
}

// WriteArchiveList imprime a lista de entries (de index ou --search).
func WriteArchiveList(w io.Writer, entries []ArchiveEntry, searchQuery string) {
	if searchQuery != "" {
		fmt.Fprintf(w, "search results for %q (%d matches, limit %d)\n", searchQuery, len(entries), archiveSearchLimit)
	} else {
		fmt.Fprintf(w, "archive for session (%d entries)\n", len(entries))
	}
	for _, e := range entries {
		fmt.Fprintf(w, "  %s  %-8s  %6d bytes  %s\n",
			e.ID, e.Tool, e.Bytes, e.At.Format(time.RFC3339))
	}
}