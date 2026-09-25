package memory

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// helper: grava JSONL de pages e retorna path.
func writeLintFixture(t *testing.T, entries []pageEntry) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "memory_pages.jsonl")
	f, err := os.Create(path)
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
	return path
}

func TestLintJSONLVazio(t *testing.T) {
	path := writeLintFixture(t, nil)
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	if res.Pages != 0 {
		t.Fatalf("esperava 0 pages, obtive %d", res.Pages)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("esperava 0 issues, obtive %+v", res.Issues)
	}
	if !bytes.Contains(out.Bytes(), []byte("OK")) {
		t.Fatalf("output deveria ter OK: %s", out.String())
	}
}

func TestLintFrontmatterValido(t *testing.T) {
	// 3 pages com refs cruzadas formando grafo conectado:
	// A-1 -> ref/foo, ref/foo -> ref/bar, ref/bar -> A-1
	path := writeLintFixture(t, []pageEntry{
		{Path: "adr/A-1", Body: "---\nscope: project\nexpires_at: 2026-12-31\n---\nSee `path: ref/foo`"},
		{Path: "ref/foo", Body: "---\nscope: project\n---\nSee `path: ref/bar`"},
		{Path: "ref/bar", Body: "---\nscope: project\n---\nSee `path: adr/A-1`"},
	})
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	if len(res.Issues) != 0 {
		t.Fatalf("esperava 0 issues (frontmatter valido + grafo conectado), obtive: %+v", res.Issues)
	}
}

func TestLintFrontmatterInvalido(t *testing.T) {
	path := writeLintFixture(t, []pageEntry{
		{Path: "bad/scope", Body: "---\nscope: world\n---\nx"},
		{Path: "bad/exp", Body: "---\nexpires_at: not-a-date\n---\ny"},
		{Path: "bad/yaml", Body: "---\n  : malformed\n---\nz"},
	})
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	if len(res.Issues) < 2 {
		t.Fatalf("esperava >=2 issues (scope+expires_at), obtive %d: %+v", len(res.Issues), res.Issues)
	}
	hasScope := false
	hasExp := false
	for _, iss := range res.Issues {
		if iss.Type == "frontmatter_invalid" && iss.Page == "bad/scope" {
			hasScope = true
		}
		if iss.Type == "frontmatter_invalid" && iss.Page == "bad/exp" {
			hasExp = true
		}
	}
	if !hasScope || !hasExp {
		t.Fatalf("esperava issues para bad/scope e bad/exp, obtive: %+v", res.Issues)
	}
}

func TestLintDanglingRef(t *testing.T) {
	path := writeLintFixture(t, []pageEntry{
		{Path: "ref/orphan-ref", Body: "---\nscope: project\n---\nSee also: `path: missing/target`"},
	})
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	// page unica com dangling_ref + orphan (esperado: ambos tipos presentes)
	var hasDangling bool
	for _, iss := range res.Issues {
		if iss.Type == "dangling_ref" && iss.Detail == `path: "missing/target" nao existe no JSONL` {
			hasDangling = true
		}
	}
	if !hasDangling {
		t.Fatalf("esperava dangling_ref para missing/target, obtive: %+v", res.Issues)
	}
}

func TestLintOrphan(t *testing.T) {
	path := writeLintFixture(t, []pageEntry{
		{Path: "ref/source", Body: "---\nscope: project\n---\nText"},
		{Path: "ref/unused", Body: "---\nscope: project\n---\nOther text"},
	})
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	// ref/unused eh orphan (ninguem referencia via path:)
	// ref/source tambem eh orphan (eh o "source" mas nao eh referenciado)
	orphanCount := 0
	for _, iss := range res.Issues {
		if iss.Type == "orphan" {
			orphanCount++
		}
	}
	if orphanCount != 2 {
		t.Fatalf("esperava 2 orphans, obtive %d: %+v", orphanCount, res.Issues)
	}
}

func TestLintReferencedNotOrphan(t *testing.T) {
	path := writeLintFixture(t, []pageEntry{
		{Path: "ref/target", Body: "---\nscope: project\n---\nTarget content"},
		{Path: "ref/source", Body: "---\nscope: project\n---\nSee `path: ref/target`"},
	})
	var out bytes.Buffer
	res, err := RunLint(path, &out, false)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	// ref/target eh referenciado por source -> NAO orphan
	// ref/source NAO eh referenciado -> orphan
	if len(res.Issues) != 1 || res.Issues[0].Type != "orphan" || res.Issues[0].Page != "ref/source" {
		t.Fatalf("esperava 1 orphan (ref/source), obtive: %+v", res.Issues)
	}
}

func TestLintJSONOutput(t *testing.T) {
	path := writeLintFixture(t, []pageEntry{
		{Path: "x", Body: "---\nscope: project\n---\ny"},
	})
	var out bytes.Buffer
	_, err := RunLint(path, &out, true)
	if err != nil {
		t.Fatalf("RunLint: %v", err)
	}
	var got LintResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON unmarshal: %v (output=%s)", err, out.String())
	}
	if got.Pages != 1 {
		t.Fatalf("esperava 1 page, obtive %d", got.Pages)
	}
}