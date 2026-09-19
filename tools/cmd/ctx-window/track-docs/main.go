// Package trackdocs monitors public CLI documentation for changes that would
// unlock the pending K-tracking-only nudge in Cursor and Antigravity CLI.
//
// Sources checked (each in parallel, 10s timeout each):
//
//  1. Cursor hooks docs HTML — https://cursor.com/docs/hooks
//  2. Cursor forum feature request #147216 — Discourse JSON endpoint
//     https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216.json
//  3. Antigravity CLI hooks docs HTML — https://antigravity.google/docs/hooks?tab=cli
//  4. Antigravity SDK Python GitHub issues — relevant enhancement issues
//
// Exit codes:
//
//	0  nothing relevant — tracking-only remains valid
//	1  positive match — token field surfaced in hook payload (reopen signal)
//	2  network error (not blocking; snapshot still written if any source replied)
//
// Snapshots are written to ~/.cache/agent-sync/docs-snapshots/<source>-<ts>.md
package trackdocs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	netTimeoutCode = 2
	matchCode      = 1
	noMatchCode    = 0
)

// Run is the entry point invoked by ctx-window's main dispatcher when the
// subcommand is "track-docs". Returns an error whose .exit-like semantics are
// encoded by the integer the binary exits with; we surface that via the
// returned intCode() helper below for tests. Production callers (main.go)
// translate the error string to an os.Exit() code.
func Run(args []string, stdout, stderr io.Writer) error {
	fs := flagNew(args)
	snapshotDir := snapshotDirFromFS(fs)
	if snapshotDir == "" {
		snapshotDir = defaultSnapshotDir()
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout*2)
	defer cancel()

	results := fetchAll(ctx, snapshotDir, stderr)
	report(stdout, results)

	if anyMatch(results) {
		return newExitError(matchCode, "positive match detected in at least one source — see snapshots and stdout")
	}
	if anyNetError(results) {
		return newExitError(netTimeoutCode, "network error on one or more sources — see stderr")
	}
	return nil
}

// Source describes one external document/endpoint we monitor.
type Source struct {
	Name     string
	URL      string
	Positive []*regexp.Regexp // match = reopen signal
	Negative []*regexp.Regexp // match = known false-positive we want to ignore in logs
}

// Result is the outcome for one Source.
type Result struct {
	Source  Source
	Body    string
	Match   bool
	Hits    []string // matched substrings
	Err     error
	WroteTo string // path of snapshot file (may be empty if no body)
}

// sources are the 4 monitored external surfaces. Regexes are tuned to avoid
// the false-positive we observed on 2026-09-10 in Cursor forum post #6:
// deanrie wrote "we've shipped other improvements to hook payloads" — that
// refers to model fields, not tokens. Generic \bshipped\b would false-match.
var sources = []Source{
	{
		Name: "cursor-hooks-docs",
		URL:  "https://cursor.com/docs/hooks",
		Positive: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(added|now\s+includes?|includes?)\s+.*tokens?\s+.*(hook\s+payload|payload\s+for\s+hook|in\s+post\s*tool\s*use|in\s+hook\s+input)`),
			regexp.MustCompile(`(?i)tokens?(input|output)?(_?metadata|_?count|_?fields?).*?(post|hook)\s*tool\s*use`),
		},
	},
	{
		Name: "cursor-forum-147216",
		URL:  "https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216.json",
		Positive: []*regexp.Regexp{
			regexp.MustCompile(`(?i)we('ve|\s+have)\s+added\s+token\s+(usage|metadata|fields|count|breakdown)`),
			regexp.MustCompile(`(?i)tokens?\s+are\s+now\s+(included|available|included\s+in\s+hook\s+payloads)`),
			regexp.MustCompile(`(?i)(implemented|delivered|shipped)\s+token\s+(usage|metadata|fields|count|breakdown)`),
		},
		// Generic "shipped" alone is a known false-positive (see post #6 by
		// deanrie referring to model field improvements). We keep it
		// available as a Negative for tests but do not include in Positive.
		Negative: []*regexp.Regexp{
			regexp.MustCompile(`(?i)we('ve|\s+have)\s+shipped\s+other\s+improvements`),
		},
	},
	{
		Name: "antigravity-hooks-docs",
		URL:  "https://antigravity.google/docs/hooks?tab=cli",
		Positive: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(added|now\s+includes?|includes?)\s+.*tokens?\s+.*(hook\s+payload|payload\s+for\s+hook|in\s+post\s*tool\s*use|in\s+hook\s+input)`),
			regexp.MustCompile(`(?i)tokens?(input|output)?(_?metadata|_?count|_?fields?).*?(post|hook)\s*tool\s*use`),
		},
	},
	{
		Name: "antigravity-sdk-issues",
		URL:  "https://api.github.com/repos/google-antigravity/antigravity-sdk-python/issues?state=open&labels=enhancement&per_page=30",
		Positive: []*regexp.Regexp{
			regexp.MustCompile(`(?i)token\s+(usage|count|budget)\s+(in|exposed\s+via|via)\s+(cli\s+hook|hook\s+payload)`),
			regexp.MustCompile(`(?i)(cli\s+hook\s+payload|hook\s+payload).*token\s+(usage|count|metadata)`),
		},
	},
}

// fetchAll hits each source in parallel with per-source timeout. On error for
// a given source we still record it; a network failure on one should not
// cancel the others.
func fetchAll(ctx context.Context, snapshotDir string, stderr io.Writer) []Result {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		out  = make([]Result, 0, len(sources))
		cli  = &http.Client{Timeout: defaultTimeout}
		etag = loadEtagState()
	)
	for _, s := range sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			r := fetchOne(ctx, cli, s, snapshotDir, etag)
			mu.Lock()
			out = append(out, r)
			mu.Unlock()
		}(s)
	}
	wg.Wait()
	return out
}

func fetchOne(ctx context.Context, cli *http.Client, s Source, snapshotDir string, etag map[string]string) Result {
	r := Result{Source: s}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		r.Err = err
		return r
	}
	req.Header.Set("User-Agent", "agent-sync-track-docs/1.0 (+https://github.com/matheusbbdutra/agent-sync)")
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.5")
	resp, err := cli.Do(req)
	if err != nil {
		r.Err = err
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		r.Err = fmt.Errorf("HTTP %d from %s", resp.StatusCode, s.URL)
		return r
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap 1MB
	if err != nil {
		r.Err = err
		return r
	}
	r.Body = string(body)
	r.Match, r.Hits = matchRegexes(s, r.Body)

	// write snapshot (always, even on match — useful for human review)
	ts := time.Now().UTC().Format("20060102T150405Z")
	path := filepath.Join(snapshotDir, s.Name+"-"+ts+".md")
	if err := os.MkdirAll(snapshotDir, 0o755); err == nil {
		header := fmt.Sprintf("# Snapshot: %s\n\nURL: %s\nFetched: %s\nStatus: HTTP %d\nMatch: %v\n\n",
			s.Name, s.URL, time.Now().UTC().Format(time.RFC3339), resp.StatusCode, r.Match)
		_ = os.WriteFile(path, []byte(header+string(body)), 0o644)
		r.WroteTo = path
	}
	return r
}

// matchRegexes reports whether any positive regex matches body and returns
// the matching substrings. It also reports (without affecting Match) any
// negative-regex hits, useful for tests to verify known false positives.
func matchRegexes(s Source, body string) (bool, []string) {
	var hits []string
	for _, re := range s.Positive {
		if loc := re.FindStringIndex(body); loc != nil {
			hits = append(hits, body[loc[0]:loc[1]])
		}
	}
	return len(hits) > 0, hits
}

// negativeHits returns all matches of Negative regexes (for tests/diagnostics).
func negativeHits(s Source, body string) []string {
	var hits []string
	for _, re := range s.Negative {
		if loc := re.FindStringIndex(body); loc != nil {
			hits = append(hits, body[loc[0]:loc[1]])
		}
	}
	return hits
}

// hasKnownFalsePositive is true when body matches a documented Negative
// regex. We do NOT suppress Match on this — we only flag it for log output
// so the human running track-docs can see the regex tolerated a known
// benign phrase.
func hasKnownFalsePositive(s Source, body string) bool {
	return len(negativeHits(s, body)) > 0
}

func report(w io.Writer, results []Result) {
	fmt.Fprintln(w, "track-docs: scanned", len(results), "sources")
	for _, r := range results {
		switch {
		case r.Err != nil:
			fmt.Fprintf(w, "  [ERR] %s: %v\n", r.Source.Name, r.Err)
		case r.Match:
			fmt.Fprintf(w, "  [MATCH] %s (%d hits):\n", r.Source.Name, len(r.Hits))
			for i, h := range r.Hits {
				if i >= 3 {
					fmt.Fprintf(w, "    ... and %d more\n", len(r.Hits)-3)
					break
				}
				fmt.Fprintf(w, "    - %s\n", truncate(h, 120))
			}
			if r.WroteTo != "" {
				fmt.Fprintf(w, "    snapshot: %s\n", r.WroteTo)
			}
		default:
			tag := "no-match"
			if hasKnownFalsePositive(r.Source, r.Body) {
				tag = "no-match (tolerated known false-positive)"
			}
			fmt.Fprintf(w, "  [OK] %s: %s\n", r.Source.Name, tag)
			if r.WroteTo != "" {
				fmt.Fprintf(w, "    snapshot: %s\n", r.WroteTo)
			}
		}
	}
}

func anyMatch(rs []Result) bool {
	for _, r := range rs {
		if r.Match {
			return true
		}
	}
	return false
}
func anyNetError(rs []Result) bool {
	for _, r := range rs {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// exitError wraps a message and an os.Exit-style code. main.go uses the
// error message text to decide the exit code via the integer returned by
// codeOf(err).
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func newExitError(code int, msg string) error {
	return &exitError{code: code, msg: msg}
}

// CodeOf returns the exit code embedded in an error produced by exitError,
// or 1 for any other error (consistent with os.Exit behavior on errors).
func CodeOf(err error) int {
	if err == nil {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return 1
}

func defaultSnapshotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "agent-sync-docs-snapshots")
	}
	return filepath.Join(home, ".cache", "agent-sync", "docs-snapshots")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// loadEtagState is a stub kept simple — we don't persist ETags yet, so this
// just returns an empty map. Hooked here so future ETag-based delta detection
// has a single seam.
func loadEtagState() map[string]string { return map[string]string{} }

// flagNew parses args. Exported for tests via parseFlags below.
type fsOpts struct {
	snapshotDir string
}

// parseFlags is a tiny helper so tests can construct fsOpts directly.
func parseFlags(args []string) fsOpts {
	var o fsOpts
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--snapshot-dir":
			if i+1 < len(args) {
				o.snapshotDir = args[i+1]
				i++
			}
		}
	}
	return o
}

// flagNew is the production parser; uses parseFlags then maps to fsOpts.
func flagNew(args []string) fsOpts { return parseFlags(args) }

// snapshotDirFromFS returns the snapshot dir override (empty if not set).
func snapshotDirFromFS(o fsOpts) string { return o.snapshotDir }

// SourceByName returns the Source config for a given name. Used by tests.
func SourceByName(name string) (Source, bool) {
	for _, s := range sources {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}

// extractPostTexts returns the concatenated text of all posts in a Discourse
// JSON response (cursor-forum-147216 source). Used by tests.
func extractPostTexts(body string) (string, error) {
	var doc struct {
		PostStream struct {
			Posts []struct {
				Cooked string `json:"cooked"`
			} `json:"posts"`
		} `json:"post_stream"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, p := range doc.PostStream.Posts {
		sb.WriteString(p.Cooked)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
