package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// GainEntry é uma linha append-only do <session>/gain.jsonl — registra uma
// compactação com chars antes/depois para calcular economia. Inspirado no
// `rtk gain` (rtk-ai/rtk, Apache-2.0): tracking SQLite com histórico de
// economia por comando. Aqui optamos por JSONL append-only (sem dependência
// extra, suficiente para casos de uso single-dev).
type GainEntry struct {
	At          time.Time `json:"at"`
	Kind        string    `json:"kind"`         // heuristic_compact | auto_compact | summarize | archive
	Version     int       `json:"version"`      // Session.Version após a compactação
	BeforeChars int       `json:"before_chars"` // EstimatedChars antes
	AfterChars  int       `json:"after_chars"`  // EstimatedChars depois
	SavedChars  int       `json:"saved_chars"`
	SavedTokens int       `json:"saved_tokens"` // ~ chars/4 (heurística rtk)
	Note        string    `json:"note,omitempty"`
}

const gainFileName = "gain.jsonl"

// gainPath devolve o path do gain.jsonl da sessão.
func gainPath(sessionID string) (string, error) {
	dir, err := SessionDir(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, gainFileName), nil
}

// AppendGainEntry grava uma entry append-only no gain.jsonl. Falha
// silenciosa: tracking é observacional, nunca pode bloquear fluxo principal.
func AppendGainEntry(sessionID string, entry GainEntry) error {
	path, err := gainPath(sessionID)
	if err != nil {
		return err
	}
	if _, err := EnsureSessionDir(sessionID); err != nil {
		return err
	}
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	if entry.SavedTokens == 0 {
		entry.SavedTokens = entry.SavedChars / 4
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// LoadGainEntries lê todas as entries do gain.jsonl. Tolerante a
// linhas corrompidas (skip silencioso). Retorna slice vazia se
// não existir.
func LoadGainEntries(sessionID string) ([]GainEntry, error) {
	path, err := gainPath(sessionID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []GainEntry
	for _, line := range bytesSplitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e GainEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// GainTotals agrega stats de uma lista de entries.
type GainTotals struct {
	Events      int
	TotalChars  int
	TotalTokens int
	ByKind      map[string]int
	FirstAt     time.Time
	LastAt      time.Time
}

func aggregateGain(entries []GainEntry) GainTotals {
	t := GainTotals{ByKind: make(map[string]int, 4)}
	if len(entries) == 0 {
		return t
	}
	t.FirstAt = entries[0].At
	t.LastAt = entries[0].At
	for _, e := range entries {
		t.Events++
		t.TotalChars += e.SavedChars
		t.TotalTokens += e.SavedTokens
		t.ByKind[e.Kind] += e.SavedChars
		if e.At.Before(t.FirstAt) {
			t.FirstAt = e.At
		}
		if e.At.After(t.LastAt) {
			t.LastAt = e.At
		}
	}
	return t
}

// WriteGainMarkdown imprime tabela markdown com entries + agregados.
func WriteGainMarkdown(w io.Writer, entries []GainEntry) {
	totals := aggregateGain(entries)
	fmt.Fprintln(w, "# ctx-window gain — savings tracking")
	fmt.Fprintln(w)
	if len(entries) == 0 {
		fmt.Fprintln(w, "(no compaction events recorded yet)")
		return
	}
	fmt.Fprintln(w, "| at | kind | version | before | after | saved_chars | saved_tokens |")
	fmt.Fprintln(w, "| --- | --- | ---: | ---: | ---: | ---: | ---: |")
	for _, e := range entries {
		fmt.Fprintf(w, "| %s | %s | %d | %d | %d | %d | %d |\n",
			e.At.Format(time.RFC3339), e.Kind, e.Version,
			e.BeforeChars, e.AfterChars, e.SavedChars, e.SavedTokens)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Totals")
	fmt.Fprintf(w, "- events: %d\n", totals.Events)
	fmt.Fprintf(w, "- total saved: %d chars (~%d tokens)\n", totals.TotalChars, totals.TotalTokens)
	if len(totals.ByKind) > 0 {
		fmt.Fprintf(w, "- by kind:\n")
		for _, kind := range []string{"heuristic_compact", "auto_compact", "summarize", "archive"} {
			if v, ok := totals.ByKind[kind]; ok {
				fmt.Fprintf(w, "  - %s: %d chars\n", kind, v)
			}
		}
	}
	if !totals.FirstAt.IsZero() {
		fmt.Fprintf(w, "- window: %s → %s\n",
			totals.FirstAt.Format(time.RFC3339), totals.LastAt.Format(time.RFC3339))
	}
}

// WriteGainJSON imprime entries + totals como JSON.
func WriteGainJSON(w io.Writer, entries []GainEntry) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"entries": entries,
		"totals":  aggregateGain(entries),
	})
}
