// Package memory: wrappers Go para as MCP tools memory_* (A-54, A-56, A-57, A-60).
//
// Origem: D-92 hygiene identificou que A-54/56/57/60 estavam como [pending]
// no STATE.md apesar de as MCP tools ja estarem wiradas em memory-mcp.
// Estes subcommands Go sao wrappers thin que operam puramente nos mesmos
// JSONLs (`memory_pages.jsonl`, `memory_feedback.jsonl`) e em
// `session-event.jsonl` via internal/event, sem cruzar para
// `tools/internal/agentmemory.Store` (cross-module bloqueado por A-59).
//
// Decisao de escopo (A-54): query faz grep simples em session-event.jsonl +
// STATE.md (sem FTS5/BM25/RRF do Store). Trade-off: menos ranking qualificado,
// mas zero deps externas. Migracao para Store fica para entrega futura se
// qualidade se mostrar insuficiente.

package memory

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/event"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// A-54 query: grep simples em session-event.jsonl + STATE.md.
// Output: JSONL com {ts, kind, note, source} por match.
type queryFlags struct {
	fs       *flag.FlagSet
	limit    int
	kind     string
	asJSON   bool
	root     string
	cacheDir string
}

func newQueryFlags(name string) *queryFlags {
	f := &queryFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.IntVar(&f.limit, "limit", 20, "maximo de matches (0 = sem limite)")
	f.fs.StringVar(&f.kind, "kind", "", "filtrar por kind")
	f.fs.BoolVar(&f.asJSON, "json", false, "saida em JSONL")
	f.fs.StringVar(&f.root, "root", "", "project root (default: derivado do binario)")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "diretorio do memory-pages JSONL")
	return f
}

func runMemoryQuery(args []string) error {
	f := newQueryFlags("agent-sync memory query")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("uso: agent-sync memory query <text> [-kind K] [-limit N]")
	}
	text := strings.TrimSpace(rest[0])
	if text == "" {
		return fmt.Errorf("query vazia")
	}

	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}

	// 1) session-event.jsonl via internal/event (mesmo caminho de recent)
	events, err := event.ReadEvents(root, event.EventReadOptions{
		Last: f.limit, Kind: f.kind,
	})
	if err != nil {
		return fmt.Errorf("ler session-events: %w", err)
	}
	matches := 0
	for _, e := range events {
		hay := e.Title + " " + e.Kind
		if !strings.Contains(strings.ToLower(hay), strings.ToLower(text)) {
			continue
		}
		if f.asJSON {
			b, _ := json.Marshal(e)
			fmt.Println(string(b))
		} else {
			fmt.Printf("[%s] %s — %s\n", e.Kind, e.TS.Format(time.RFC3339), e.Title)
		}
		matches++
		if f.limit > 0 && matches >= f.limit {
			break
		}
	}
	if !f.asJSON {
		fmt.Fprintf(os.Stderr, "query: %d match em session-events\n", matches)
	}
	return nil
}

// A-56 read-page: le memory_pages.jsonl por path, retorna body + frontmatter parseado.
type readPageFlags struct {
	fs       *flag.FlagSet
	asJSON   bool
	cacheDir string
}

func newReadPageFlags(name string) *readPageFlags {
	f := &readPageFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.BoolVar(&f.asJSON, "json", false, "saida em JSON")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "diretorio do JSONL")
	return f
}

type readPageResult struct {
	Path       string            `json:"path"`
	Scope      string            `json:"scope,omitempty"`
	ExpiresAt  string            `json:"expires_at,omitempty"`
	Body       string            `json:"body"`
	Frontmatter map[string]string `json:"frontmatter,omitempty"`
	TS         string            `json:"ts"`
}

func runMemoryReadPage(args []string) error {
	f := newReadPageFlags("agent-sync memory read-page")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("uso: agent-sync memory read-page <path>")
	}
	path := strings.TrimSpace(rest[0])
	if path == "" {
		return fmt.Errorf("path vazio")
	}
	jsonlPath, err := resolveJSONLPath(f.cacheDir)
	if err != nil {
		return err
	}
	res, err := readPageByPath(jsonlPath, path)
	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("page %q nao encontrada", path)
	}
	if f.asJSON {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("path:       %s\n", res.Path)
	if res.Scope != "" {
		fmt.Printf("scope:      %s\n", res.Scope)
	}
	if res.ExpiresAt != "" {
		fmt.Printf("expires_at: %s\n", res.ExpiresAt)
	}
	fmt.Printf("ts:         %s\n", res.TS)
	fmt.Println("----")
	fmt.Print(res.Body)
	return nil
}

// readPageByPath faz a leitura crua (sem flag parsing) — reutilizavel por testes.
func readPageByPath(jsonlPath, path string) (*readPageResult, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("abrir %s: %w", jsonlPath, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e pageEntry
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			continue
		}
		if e.Path == path {
			fm, _ := extractFrontmatter(e.Body)
			res := &readPageResult{
				Path: e.Path, Body: e.Body, TS: e.TS.Format(time.RFC3339),
				Frontmatter: fm,
			}
			if fm != nil {
				res.Scope = fm["scope"]
				res.ExpiresAt = fm["expires_at"]
			}
			return res, nil
		}
	}
	return nil, sc.Err()
}

// A-57 read-session: wrapper sobre event read -session ID.
type readSessionFlags struct {
	fs       *flag.FlagSet
	limit    int
	asJSON   bool
	root     string
	cacheDir string
}

func newReadSessionFlags(name string) *readSessionFlags {
	f := &readSessionFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.IntVar(&f.limit, "limit", 50, "maximo de eventos (0 = sem limite)")
	f.fs.BoolVar(&f.asJSON, "json", false, "saida em JSONL")
	f.fs.StringVar(&f.root, "root", "", "project root")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "diretorio")
	return f
}

func runMemoryReadSession(args []string) error {
	f := newReadSessionFlags("agent-sync memory read-session")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("uso: agent-sync memory read-session -session <id>")
	}
	session := strings.TrimSpace(rest[0])
	if session == "" {
		return fmt.Errorf("session_id vazio")
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	// source: usa -cache-dir (JSONL memory) se setado, senao session-event do root
	var events []event.SessionEvent
	if f.cacheDir != "" {
		events, err = readEventsFromJSONL(filepath.Join(f.cacheDir, "memory.jsonl"), session, f.limit)
	} else {
		events, err = readSessionEvents(root, session, f.limit)
	}
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("nenhum evento para session_id %q", session)
	}
	for _, e := range events {
		if f.asJSON {
			b, _ := json.Marshal(e)
			fmt.Println(string(b))
		} else {
			fmt.Printf("[%s] %s — %s\n", e.Kind, e.TS.Format(time.RFC3339), e.Title)
		}
	}
	return nil
}

// readSessionEvents le session-events.jsonl e filtra por session_id.
// Usa bufio.Scanner manual porque event.ReadEvents nao expoe filtro por session.
func readSessionEvents(root, sessionID string, limit int) ([]event.SessionEvent, error) {
	path := filepath.Join(root, ".agent-sync", "session-events.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []event.SessionEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e event.SessionEvent
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			continue
		}
		if e.SessionID == sessionID {
			out = append(out, e)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, sc.Err()
}

// readEventsFromJSONL le qualquer JSONL de eventos (memory.jsonl ou similar) e filtra por session.
func readEventsFromJSONL(path, sessionID string, limit int) ([]event.SessionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []event.SessionEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e event.SessionEvent
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			continue
		}
		if e.SessionID == sessionID {
			out = append(out, e)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, sc.Err()
}

// A-60 delete-page: remove entry do JSONL por path. Idempotente.
type deletePageFlags struct {
	fs       *flag.FlagSet
	asJSON   bool
	dryRun   bool
	cacheDir string
}

func newDeletePageFlags(name string) *deletePageFlags {
	f := &deletePageFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.BoolVar(&f.asJSON, "json", false, "saida em JSON")
	f.fs.BoolVar(&f.dryRun, "dry-run", true, "so mostra o que seria removido")
	f.fs.StringVar(&f.cacheDir, "cache-dir", "", "diretorio")
	return f
}

func runMemoryDeletePage(args []string) error {
	f := newDeletePageFlags("agent-sync memory delete-page")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	rest := f.fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("uso: agent-sync memory delete-page <path>")
	}
	path := strings.TrimSpace(rest[0])
	if path == "" {
		return fmt.Errorf("path vazio")
	}
	jsonlPath, err := resolveJSONLPath(f.cacheDir)
	if err != nil {
		return err
	}
	removed, kept, err := deletePageInJSONL(jsonlPath, path, f.dryRun)
	if err != nil {
		return err
	}
	if f.asJSON {
		data, _ := json.MarshalIndent(map[string]int{"removed": removed, "kept": kept}, "", "  ")
		fmt.Println(string(data))
	} else {
		if f.dryRun {
			fmt.Printf("delete-page (dry-run): %d entry(s) seriam removidas, %d mantidas\n", removed, kept)
		} else {
			fmt.Printf("delete-page: %d removida(s), %d mantidas\n", removed, kept)
		}
	}
	return nil
}

// deletePageInJSONL remove entry com path matching. dryRun=true NAO reescreve.
func deletePageInJSONL(jsonlPath, path string, dryRun bool) (removed, kept int, err error) {
	in, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	defer in.Close()

	type pair struct {
		line string
		matched bool
	}
	var entries []pair
	var order []int // indices of entries that survived
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e pageEntry
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			// mantem linha malformada (nao eh a page alvo)
			entries = append(entries, pair{line: text, matched: false})
			order = append(order, len(entries)-1)
			kept++
			continue
		}
		if e.Path == path {
			entries = append(entries, pair{line: text, matched: true})
			removed++
		} else {
			entries = append(entries, pair{line: text, matched: false})
			order = append(order, len(entries)-1)
			kept++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	if dryRun || removed == 0 {
		return removed, kept, nil
	}
	// rewrite (atomic: tmp + rename)
	out, err := os.Create(jsonlPath + ".tmp")
	if err != nil {
		return 0, 0, fmt.Errorf("criar tmp: %w", err)
	}
	defer out.Close()
	for _, idx := range order {
		if _, err := fmt.Fprintln(out, entries[idx].line); err != nil {
			return 0, 0, err
		}
	}
	if err := os.Rename(jsonlPath+".tmp", jsonlPath); err != nil {
		return 0, 0, fmt.Errorf("rename: %w", err)
	}
	return removed, kept, nil
}

// sortMapKeys utility (nao reusar o do lint.go por escopo separado).
func sortMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ensure imports used
var _ = io.EOF