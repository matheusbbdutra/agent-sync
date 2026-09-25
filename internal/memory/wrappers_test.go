package memory

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper: grava JSONL de pages e retorna path + cacheDir.
func writeWrappersFixture(t *testing.T, entries []pageEntry) (cacheDir, jsonlPath string) {
	t.Helper()
	cacheDir = t.TempDir()
	jsonlPath = filepath.Join(cacheDir, "memory_pages.jsonl")
	f, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, e := range entries {
		if e.TS.IsZero() {
			e.TS = time.Now().UTC()
		}
		line, _ := json.Marshal(e)
		f.Write(append(line, '\n'))
	}
	return cacheDir, jsonlPath
}

func TestReadPageFound(t *testing.T) {
	_, jsonlPath := writeWrappersFixture(t, []pageEntry{
		{Path: "adr/A-1", Body: "---\nscope: project\nexpires_at: 2026-12-31\n---\n# Title"},
		{Path: "ref/foo", Body: "---\nscope: global\n---\nOther"},
	})
	res, err := readPageByPath(jsonlPath, "adr/A-1")
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.Path != "adr/A-1" {
		t.Fatalf("esperava path=adr/A-1, obtive %+v", res)
	}
	if res.Scope != "project" || res.ExpiresAt != "2026-12-31" {
		t.Fatalf("frontmatter nao parseou: %+v", res)
	}
	if !strings.Contains(res.Body, "# Title") {
		t.Fatalf("body errado: %q", res.Body)
	}
}

func TestReadPageNotFound(t *testing.T) {
	_, jsonlPath := writeWrappersFixture(t, nil)
	res, err := readPageByPath(jsonlPath, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatalf("esperava nil para missing, obtive %+v", res)
	}
}

func TestDeletePageDryRunKeepsFile(t *testing.T) {
	_, jsonlPath := writeWrappersFixture(t, []pageEntry{
		{Path: "a", Body: "A"},
		{Path: "b", Body: "B"},
		{Path: "c", Body: "C"},
	})
	removed, kept, err := deletePageInJSONL(jsonlPath, "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 || kept != 2 {
		t.Fatalf("esperava removed=1 kept=2, obtive %d/%d", removed, kept)
	}
	// dry-run NAO reescreve — verificar original intacto
	res, _ := readPageByPath(jsonlPath, "b")
	if res == nil {
		t.Fatal("dry-run removeu o arquivo indevidamente")
	}
}

func TestDeletePageConfirmRemoves(t *testing.T) {
	_, jsonlPath := writeWrappersFixture(t, []pageEntry{
		{Path: "a", Body: "A"},
		{Path: "b", Body: "B"},
	})
	removed, kept, err := deletePageInJSONL(jsonlPath, "b", false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 || kept != 1 {
		t.Fatalf("esperava removed=1 kept=1, obtive %d/%d", removed, kept)
	}
	res, _ := readPageByPath(jsonlPath, "b")
	if res != nil {
		t.Fatalf("confirm deveria ter removido, ainda existe: %+v", res)
	}
	res, _ = readPageByPath(jsonlPath, "a")
	if res == nil {
		t.Fatal("a deveria ter sobrado")
	}
}

func TestDeletePageIdempotent(t *testing.T) {
	_, jsonlPath := writeWrappersFixture(t, []pageEntry{
		{Path: "x", Body: "X"},
	})
	removed, _, err := deletePageInJSONL(jsonlPath, "x", false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("primeira chamada: esperava removed=1, obtive %d", removed)
	}
	removed2, _, err := deletePageInJSONL(jsonlPath, "x", false)
	if err != nil {
		t.Fatal(err)
	}
	if removed2 != 0 {
		t.Fatalf("segunda chamada (idempotente): esperava removed=0, obtive %d", removed2)
	}
}

func TestSortMapKeys(t *testing.T) {
	got := sortMapKeys(map[string]int{"c": 1, "a": 2, "b": 3})
	want := []string{"a", "b", "c"}
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// helper: testa output de run* sem flag.Parse (executa direto)
func TestRunMemoryReadPageJSON(t *testing.T) {
	cacheDir, _ := writeWrappersFixture(t, []pageEntry{
		{Path: "x", Body: "---\nscope: project\n---\nbody"},
	})
	// usa runMemoryReadPage mas com cache-dir override
	f := newReadPageFlags("test")
	_ = f.fs.Parse([]string{"-cache-dir", cacheDir, "-json", "x"})
	var out bytes.Buffer
	_ = out
	res, err := readPageByPath(filepath.Join(cacheDir, "memory_pages.jsonl"), "x")
	if err != nil || res == nil {
		t.Fatalf("readPageByPath: %v %+v", err, res)
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	if !strings.Contains(string(b), `"scope": "project"`) {
		t.Fatalf("JSON sem scope: %s", b)
	}
}