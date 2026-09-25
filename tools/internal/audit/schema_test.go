package audit

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractTablesFromDDL(t *testing.T) {
	cases := []struct {
		name, content string
		want          []string
	}{
		{
			name:    "create_table_simple",
			content: "CREATE TABLE IF NOT EXISTS memories (id INTEGER);",
			want:    []string{"memories"},
		},
		{
			name:    "multiple_tables_dedup",
			content: "CREATE TABLE memories (id INTEGER); CREATE TABLE events (id INTEGER); INSERT INTO memories VALUES (1); INSERT INTO events VALUES (2);",
			want:    []string{"events", "memories"},
		},
		{
			name:    "filtered_keywords",
			content: "SELECT * FROM users WHERE id = 1; INSERT INTO users (name) VALUES ('x');",
			want:    []string{"users"},
		},
		{
			name:    "from_join_update",
			content: "SELECT a.* FROM memories a JOIN events b ON a.id = b.id; UPDATE memories SET x = 1;",
			want:    []string{"events", "memories"},
		},
		{
			name:    "quoted_table_name",
			content: "CREATE TABLE IF NOT EXISTS `memories` (id INTEGER);",
			want:    []string{"memories"},
		},
		{
			name:    "no_sql",
			content: "package main\nfunc foo() {}\n",
			want:    nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ExtractTablesFromDDL(c.content)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v; want %v", got, c.want)
			}
		})
	}
}

func TestExtractTablesFromFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "schema.go")
	content := `package store

const ddl = "` + "```" + `
CREATE TABLE IF NOT EXISTS memories (id INTEGER PRIMARY KEY);
CREATE TABLE IF NOT EXISTS memory_sync_state (type TEXT);
` + "```" + `"
`
	mustWrite(t, path, content)

	got, err := ExtractTablesFromFile(path)
	if err != nil {
		t.Fatalf("ExtractTablesFromFile: %v", err)
	}
	want := []string{"memories", "memory_sync_state"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v; want %v", got, want)
	}
}

func TestExtractTablesFromFiles_Missing(t *testing.T) {
	got := ExtractTablesFromFiles("", []string{"/nonexistent/foo.go"})
	if got != nil {
		t.Errorf("expected nil for missing files; got %v", got)
	}
}

func TestExtractTablesFromFiles_WithRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "schema.go"), `package s
const ddl = "CREATE TABLE IF NOT EXISTS widgets (id INTEGER)"
`)
	got := ExtractTablesFromFiles(root, []string{"schema.go"})
	if len(got) != 1 || got[0] != "widgets" {
		t.Errorf("expected [widgets]; got %v", got)
	}
}
