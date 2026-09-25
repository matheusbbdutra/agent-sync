package skills

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	fnErr := fn()
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	return buf.String(), fnErr
}

func setupSkillsFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()

	skillsDir := filepath.Join(base, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"a", "b", "orphan"} {
		dir := filepath.Join(skillsDir, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		skill := "---\nname: " + id + "\ndescription: fixture\n---\n# " + id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	manifest := `{
  "source": {
    "repository": "https://example.com/fixture",
    "commit": "deadbeef",
    "license": "MIT",
    "skillsDir": "~/skills-fixture"
  },
  "skills": [
    { "id": "a", "domain": "domain-a", "bundleRef": "core-dev" },
    { "id": "b", "domain": "domain-b", "bundleRef": ["core-dev", "security-core"] },
    { "id": "c", "domain": "domain-c" }
  ]
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	return base
}

func TestRunSkillsIndexTextOutput(t *testing.T) {
	base := setupSkillsFixture(t)

	out, err := withStdout(t, func() error {
		return RunIndexWithBase(nil, base)
	})
	if err != nil {
		t.Fatalf("RunIndex text: %v", err)
	}

	wantSubstrings := []string{
		"Skills index",
		"installed=", "missing=", "orphan=",
		"a",
		"b",
		"c",
		"orphan",
		"STATUS",
		"installed", "missing", "orphan",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Errorf("text output missing %q\n--- output ---\n%s", s, out)
		}
	}
}

func TestRunSkillsIndexJSONOutput(t *testing.T) {
	base := setupSkillsFixture(t)

	out, err := withStdout(t, func() error {
		return RunIndexWithBase([]string{"-json"}, base)
	})
	if err != nil {
		t.Fatalf("RunIndex json: %v", err)
	}

	var r SkillIndexResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON output: %v\n--- output ---\n%s", err, out)
	}

	if r.Total != 4 {
		t.Errorf("Total: got %d, want 4 (a, b, c, orphan)", r.Total)
	}
	if r.Installed != 2 {
		t.Errorf("Installed: got %d, want 2 (a, b)", r.Installed)
	}
	if r.Missing != 1 {
		t.Errorf("Missing: got %d, want 1 (c)", r.Missing)
	}
	if r.Orphan != 1 {
		t.Errorf("Orphan: got %d, want 1 (orphan)", r.Orphan)
	}

	for _, s := range r.Skills {
		if s.ID == "b" {
			if len(s.BundleRef) != 2 {
				t.Errorf("b.BundleRef len: got %d, want 2 (%v)", len(s.BundleRef), s.BundleRef)
			}
		}
		if s.ID == "orphan" {
			if s.Status != StatusOrphan {
				t.Errorf("orphan.Status: got %q, want %q", s.Status, StatusOrphan)
			}
		}
	}
}

func TestRunSkillsIndexOrphansFilter(t *testing.T) {
	base := setupSkillsFixture(t)

	out, err := withStdout(t, func() error {
		return RunIndexWithBase([]string{"-json", "-orphans"}, base)
	})
	if err != nil {
		t.Fatalf("RunIndex orphans: %v", err)
	}

	var r SkillIndexResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON output: %v\n--- output ---\n%s", err, out)
	}

	if r.Orphan != 1 || r.Total != 1 {
		t.Errorf("filtro --orphans: Total=%d Orphan=%d, want 1/1", r.Total, r.Orphan)
	}
	if r.Skills[0].ID != "orphan" {
		t.Errorf("filtro --orphans retornou id errado: %q", r.Skills[0].ID)
	}
}

func TestRunSkillsIndexMissingFilter(t *testing.T) {
	base := setupSkillsFixture(t)

	out, err := withStdout(t, func() error {
		return RunIndexWithBase([]string{"-json", "-missing"}, base)
	})
	if err != nil {
		t.Fatalf("RunIndex missing: %v", err)
	}

	var r SkillIndexResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON output: %v\n--- output ---\n%s", err, out)
	}

	if r.Missing != 1 || r.Total != 1 {
		t.Errorf("filtro --missing: Total=%d Missing=%d, want 1/1", r.Total, r.Missing)
	}
	if r.Skills[0].ID != "c" {
		t.Errorf("filtro --missing retornou id errado: %q", r.Skills[0].ID)
	}
}

func TestRunSkillsIndexRealManifest(t *testing.T) {
	out, err := withStdout(t, func() error {
		return RunIndex([]string{"-json"})
	})
	if err != nil {
		t.Fatalf("RunIndex real: %v", err)
	}

	var r SkillIndexResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	const expectedTotal = 54
	const expectedInstalled = 54
	if r.Total != expectedTotal {
		t.Errorf("Total=%d, esperado %d (cobertura total)", r.Total, expectedTotal)
	}
	if r.Installed != expectedInstalled {
		t.Errorf("Installed=%d, esperado %d (todas declaradas no manifest)", r.Installed, expectedInstalled)
	}
	if r.Missing != 0 {
		t.Errorf("Missing=%d, esperado 0", r.Missing)
	}
	if r.Orphan != 0 {
		t.Errorf("Orphan=%d, esperado 0", r.Orphan)
	}
	t.Logf("real manifest: total=%d installed=%d missing=%d orphan=%d",
		r.Total, r.Installed, r.Missing, r.Orphan)
}

func TestBundleRefToSlice(t *testing.T) {
	cases := []struct {
		in   any
		want []string
	}{
		{nil, nil},
		{"", nil},
		{"core-dev", []string{"core-dev"}},
		{[]any{"a", "b"}, []string{"a", "b"}},
		{[]any{"a", "", "c"}, []string{"a", "c"}},
		{42, nil},
	}
	for _, c := range cases {
		got := BundleRefToSlice(c.in)
		if !equalStrings(got, c.want) {
			t.Errorf("BundleRefToSlice(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
