package audit

import (
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_Basic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	mustWrite(t, path, `---
status: archived
tags: [old, deprecated]
archived_at: 2024-01-15
---

# Heading

body content
`)

	fm, body, err := ParseFrontmatter(path)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm == nil {
		t.Fatal("expected frontmatter; got nil")
	}
	if fm.Status != "archived" {
		t.Errorf("Status = %q; want archived", fm.Status)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "old" || fm.Tags[1] != "deprecated" {
		t.Errorf("Tags = %v; want [old deprecated]", fm.Tags)
	}
	if fm.ArchivedAt != "2024-01-15" {
		t.Errorf("ArchivedAt = %q; want 2024-01-15", fm.ArchivedAt)
	}
	if !fm.IsHistorical() {
		t.Error("IsHistorical should be true for status=archived")
	}
	if len(body) == 0 {
		t.Error("body should not be empty")
	} else {
		// após o "---" pode haver uma linha em branco antes do heading.
		trimmed := body
		for len(trimmed) > 0 && (trimmed[0] == '\n' || trimmed[0] == '\r') {
			trimmed = trimmed[1:]
		}
		if len(trimmed) == 0 || trimmed[0] != '#' {
			t.Errorf("body should start (after whitespace) with '#' heading; got %q", trimmed[:min(20, len(trimmed))])
		}
	}
}

func TestParseFrontmatter_None(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "plain.md")
	mustWrite(t, path, "# Just a heading\n\nno frontmatter\n")
	fm, body, err := ParseFrontmatter(path)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm != nil {
		t.Errorf("expected nil frontmatter; got %+v", fm)
	}
	if len(body) == 0 {
		t.Error("body should not be empty")
	}
}

func TestParseFrontmatter_ActiveDoc(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "active.md")
	mustWrite(t, path, `---
status: active
tags: [proposed]
---

# Active ADR
`)
	fm, _, err := ParseFrontmatter(path)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm.IsHistorical() {
		t.Error("IsHistorical should be false for status=active")
	}
}

func TestParseFrontmatter_DeprecatedTag(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tagged.md")
	mustWrite(t, path, `---
tags: [deprecated]
---
`)
	fm, _, err := ParseFrontmatter(path)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !fm.IsHistorical() {
		t.Error("IsHistorical should be true for tag=deprecated")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
