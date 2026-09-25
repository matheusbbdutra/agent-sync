package agentmemory

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSummaryDecisionsAndNextSteps(t *testing.T) {
	yaml := `decisions:
  - "use LLM cascade"
  - "default summarizer = own model"
next_steps:
  - "calibrate K"
  - "wire memory-mcp"
`
	got := ParseSummary(yaml)
	if len(got) != 4 {
		t.Fatalf("expected 4 records, got %d: %+v", len(got), got)
	}
	want := []string{"decisions", "decisions", "next_steps", "next_steps"}
	for i, w := range want {
		if got[i].Section != w {
			t.Errorf("record[%d].Section = %q, want %q", i, got[i].Section, w)
		}
	}
	if got[0].Content != "use LLM cascade" {
		t.Errorf("record[0].Content = %q, want %q", got[0].Content, "use LLM cascade")
	}
}

func TestParseSummaryArtifactsAndErrors(t *testing.T) {
	yaml := `artifacts:
  - path: "tools/cmd/ctx-window/main.go"
    description: "entrypoint"
resolved_errors:
  - cause: "missing schema"
    fix: "add struct tag"
`
	got := ParseSummary(yaml)
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(got), got)
	}
	if got[0].Section != "artifacts" {
		t.Errorf("record[0].Section = %q, want artifacts", got[0].Section)
	}
	if !strings.HasPrefix(got[0].Key, "path:") {
		t.Errorf("record[0].Key = %q, want prefix path:", got[0].Key)
	}
	if got[1].Section != "resolved_errors" {
		t.Errorf("record[1].Section = %q, want resolved_errors", got[1].Section)
	}
}

func TestParseSummaryEmptyAndMalformed(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want int
	}{
		{"empty", "", 0},
		{"only_comments", "# comment\n# another\n", 0},
		{"single_section_inline", `decisions: ["a", "b"]`, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := len(ParseSummary(c.yaml)); got != c.want {
				t.Errorf("len(ParseSummary) = %d, want %d", got, c.want)
			}
		})
	}
}

func TestParseSummaryStableKeys(t *testing.T) {
	yaml := `decisions:
  - "use LLM cascade"
`
	a := ParseSummary(yaml)
	b := ParseSummary(yaml)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("expected 1 record each, got %d/%d", len(a), len(b))
	}
	if a[0].Key != b[0].Key {
		t.Errorf("keys divergem para mesmo conteúdo: %q vs %q", a[0].Key, b[0].Key)
	}
}

func TestUpsertSummaryWritesRowsAndSearchFinds(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mem.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	yaml := `decisions:
  - "use LLM cascade for cost"
next_steps:
  - "wire memory-mcp integration"
`
	n, err := store.UpsertSummary("git:abc123", "claude-code", "sess-1", yaml)
	if err != nil {
		t.Fatalf("UpsertSummary: %v", err)
	}
	if n != 2 {
		t.Errorf("UpsertSummary n = %d, want 2", n)
	}

	items, err := store.Search("LLM cascade", "", "", 10, ScopeFilter{ProjectID: "git:abc123"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one match for LLM cascade")
	}
	if !strings.Contains(items[0].Content, "LLM cascade") {
		t.Errorf("first match content missing term: %q", items[0].Content)
	}
}

func TestUpsertSummaryIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mem.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	yaml := `decisions:
  - "stable decision text"
`
	if _, err := store.UpsertSummary("p1", "claude-code", "s1", yaml); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertSummary("p1", "codex", "s2", yaml); err != nil {
		t.Fatal(err)
	}

	items, err := store.List("", "", 100, ScopeFilter{ProjectID: "p1"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, m := range items {
		if strings.Contains(m.Content, "stable decision text") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry (overwrite by hash), got %d", count)
	}
}
