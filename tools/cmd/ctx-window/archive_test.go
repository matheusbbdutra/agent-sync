package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveIDDeterministic(t *testing.T) {
	a := archiveID("hello world")
	b := archiveID("hello world")
	if a != b {
		t.Errorf("same input should produce same id, got %q vs %q", a, b)
	}
	c := archiveID("hello WORLD")
	if c == a {
		t.Errorf("different input should produce different id, both %q", a)
	}
	if len(a) != 16 {
		t.Errorf("archiveID should be 16 hex chars (sha256[:8]), got %d", len(a))
	}
}

func TestArchiveThresholdDefault(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_ARCHIVE_AT", "")
	if got := archiveThreshold(); got != archiveDefaultAt {
		t.Errorf("default threshold=%d, want %d", got, archiveDefaultAt)
	}
	t.Setenv("AGENT_SYNC_CTX_ARCHIVE_AT", "1024")
	if got := archiveThreshold(); got != 1024 {
		t.Errorf("env threshold=1024, got %d", got)
	}
}

func TestArchiveResultBelowThreshold(t *testing.T) {
	withTempCache(t)
	entry, err := ArchiveResult("small-sess", "Read", "tiny content")
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Errorf("entry should be nil for content below threshold, got %+v", entry)
	}
}

func TestArchiveResultWritesFileAndIndex(t *testing.T) {
	withTempCache(t)
	content := strings.Repeat("x", 5000)
	entry, err := ArchiveResult("arch-sess", "Read", content)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("entry should not be nil for content above threshold")
	}
	if entry.Bytes != 5000 {
		t.Errorf("entry.Bytes=%d, want 5000", entry.Bytes)
	}
	// file deve existir
	if _, err := os.Stat(entry.Path); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
	// index deve ter 1 linha
	idx, err := LoadArchiveIndex("arch-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 1 {
		t.Fatalf("expected 1 index entry, got %d", len(idx))
	}
	if idx[0].ID != entry.ID {
		t.Errorf("index ID mismatch: %s vs %s", idx[0].ID, entry.ID)
	}
}

func TestArchiveResultDedup(t *testing.T) {
	withTempCache(t)
	content := strings.Repeat("y", 5000)
	first, _ := ArchiveResult("dedup-sess", "Bash", content)
	second, _ := ArchiveResult("dedup-sess", "Bash", content)
	if first == nil || second == nil {
		t.Fatal("both should archive")
	}
	if first.ID != second.ID {
		t.Errorf("dedup should produce same ID, got %s vs %s", first.ID, second.ID)
	}
	idx, _ := LoadArchiveIndex("dedup-sess")
	if len(idx) != 1 {
		t.Errorf("dedup should keep index at 1 entry, got %d", len(idx))
	}
}

func TestLoadArchiveIndexEmpty(t *testing.T) {
	withTempCache(t)
	idx, err := LoadArchiveIndex("none-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 0 {
		t.Errorf("empty session should have empty index, got %d", len(idx))
	}
}

func TestLoadArchiveIndexSkipsCorrupt(t *testing.T) {
	withTempCache(t)
	// setup: 1 linha válida + 1 corrompida
	dir, _ := archiveDir("corrupt-sess")
	idxPath := filepath.Join(dir, archiveIndexName)
	validEntry := ArchiveEntry{ID: "abc12345", Tool: "Read", Bytes: 5000, At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	line, _ := json.Marshal(validEntry)
	body := append(line, '\n')
	body = append(body, []byte("not json\n")...)
	body = append(body, line...)
	body = append(body, '\n')
	if err := os.WriteFile(idxPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadArchiveIndex("corrupt-sess")
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 2 {
		t.Errorf("expected 2 valid entries (corrupt skipped), got %d", len(idx))
	}
}

func TestMaybeArchiveBelowThreshold(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_ARCHIVE_AT", "100000")
	preview, id, archived := MaybeArchive("small content")
	if archived {
		t.Error("below threshold should not archive")
	}
	if preview != "small content" {
		t.Errorf("preview should equal content when below threshold, got %q", preview)
	}
	if id != "" {
		t.Errorf("id should be empty when below threshold, got %q", id)
	}
}

func TestMaybeArchiveAboveThreshold(t *testing.T) {
	t.Setenv("AGENT_SYNC_CTX_ARCHIVE_AT", "1000")
	big := strings.Repeat("z", 5000)
	preview, id, archived := MaybeArchive(big)
	if !archived {
		t.Error("above threshold should archive")
	}
	if id == "" {
		t.Error("id should not be empty when archived")
	}
	if len(preview) >= len(big) {
		t.Errorf("preview should be shorter than original (%d vs %d)", len(preview), len(big))
	}
	if !strings.Contains(preview, "[... archived:") {
		t.Errorf("preview should contain archive marker, got %q", preview)
	}
}

func TestExpandByID(t *testing.T) {
	withTempCache(t)
	content := strings.Repeat("expandable content\n", 300) // > 4KB threshold
	entry, _ := ArchiveResult("expand-sess", "Read", content)
	if entry == nil {
		t.Fatal("entry should not be nil")
	}
	body, got, err := ExpandByID("expand-sess", entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if body != content {
		t.Errorf("expanded body mismatch (len=%d vs %d)", len(body), len(content))
	}
	if got.ID != entry.ID {
		t.Errorf("entry ID mismatch")
	}
}

func TestExpandByIDNotFound(t *testing.T) {
	withTempCache(t)
	_, _, err := ExpandByID("missing-sess", "00000000")
	if err == nil {
		t.Error("expected error for nonexistent id")
	}
}

func TestSearchArchive(t *testing.T) {
	withTempCache(t)
	// conteúdos > 4KB (archiveDefaultAt) para serem arquivados
	ArchiveResult("search-sess", "Read", strings.Repeat("a", 3000)+" UNIQUE_TOKEN_AAA "+strings.Repeat("b", 3000))
	ArchiveResult("search-sess", "Read", strings.Repeat("c", 3000)+" UNIQUE_TOKEN_BBB "+strings.Repeat("d", 3000))
	ArchiveResult("search-sess", "Read", strings.Repeat("e", 8000))
	// case-insensitive
	matches, err := SearchArchive("search-sess", "unique_token_aaa")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestSearchArchiveEmpty(t *testing.T) {
	withTempCache(t)
	if _, err := SearchArchive("any", ""); err == nil {
		t.Error("expected error for empty query")
	}
}

func TestWriteArchiveList(t *testing.T) {
	var buf bytes.Buffer
	WriteArchiveList(&buf, nil, "")
	if !strings.Contains(buf.String(), "0 entries") {
		t.Errorf("empty list output unexpected: %q", buf.String())
	}
	buf.Reset()
	WriteArchiveList(&buf, nil, "foo")
	if !strings.Contains(buf.String(), `search results for "foo"`) {
		t.Errorf("search output unexpected: %q", buf.String())
	}
}

func TestRunArchiveBelowThreshold(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"archive", "--tool", "Read", "--content", "tiny", "run-small"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"archived":false`) {
		t.Errorf("expected archived=false, got %s", stdout.String())
	}
}

func TestRunArchiveAboveThreshold(t *testing.T) {
	withTempCache(t)
	big := strings.Repeat("a", 5000)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"archive", "--tool", "Bash", "--content", big, "run-big"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"archived":true`) {
		t.Errorf("expected archived=true, got %s", stdout.String())
	}
}

func TestRunExpandList(t *testing.T) {
	withTempCache(t)
	big := strings.Repeat("x", 5000)
	var setupOut, setupErr bytes.Buffer
	if err := run([]string{"archive", "--tool", "Read", "--content", big, "list-sess"}, &setupOut, &setupErr); err != nil {
		t.Fatalf("setup archive failed: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"expand", "--list", "list-sess"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "1 entries") {
		t.Errorf("expected 1 entries, got %s", stdout.String())
	}
}

func TestRunExpandMissingArgs(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"expand", "sess"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when no --list/--search/<id> given")
	}
}