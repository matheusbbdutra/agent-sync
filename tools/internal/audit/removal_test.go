package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestUpperSnake(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"memory-mcp", "MEMORY_MCP"},
		{"tools/cmd/memory-mcp", "TOOLS_CMD_MEMORY_MCP"},
		{"helloWorld", "HELLO_WORLD"},
		{"already_Snake", "ALREADY_SNAKE"},
		{"trailing--dash", "TRAILING_DASH"},
		{"", ""},
	}
	for _, c := range cases {
		got := upperSnake(c.in)
		if got != c.want {
			t.Errorf("upperSnake(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestNewTarget_BasicMatchers(t *testing.T) {
	tg, err := NewTarget("tools/cmd/memory-mcp")
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	if tg.Raw != "tools/cmd/memory-mcp" {
		t.Errorf("Raw = %q; want %q", tg.Raw, "tools/cmd/memory-mcp")
	}
	if len(tg.Matchers) < 2 {
		t.Fatalf("expected >=2 matchers; got %d", len(tg.Matchers))
	}

	// Matcher 0: literal case-insensitive
	if !tg.Matchers[0].Regex.MatchString("tools/cmd/memory-mcp/main.go") {
		t.Errorf("matcher[0] should match literal path")
	}
	if !tg.Matchers[0].Regex.MatchString("Tools/Cmd/Memory-Mcp") {
		t.Errorf("matcher[0] should be case-insensitive")
	}
	if tg.Matchers[0].Regex.MatchString("tools-other-foo") {
		t.Errorf("matcher[0] should NOT match unrelated text")
	}

	// Matcher 1: subpath @tools/cmd/memory-mcp/*
	if !tg.Matchers[1].Regex.MatchString("import @tools/cmd/memory-mcp/store") {
		t.Errorf("matcher[1] should match @<target>/<segment>")
	}
	if tg.Matchers[1].Regex.MatchString("@tools/cmd/memory-mcp/") {
		t.Errorf("matcher[1] should require segment after /")
	}

	// Matcher 2: UPPER_SNAKE prefix
	if tg.Matchers[2].Regex.MatchString("tools_cmd_memory_mcp_main") {
		t.Errorf("matcher[2] requires uppercase; got false positive on lowercase")
	}
	if !tg.Matchers[2].Regex.MatchString("TOOLS_CMD_MEMORY_MCP_TABLE") {
		t.Errorf("matcher[2] should match UPPER_SNAKE_* pattern")
	}
}

func TestNewTarget_Empty(t *testing.T) {
	_, err := NewTarget("")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
	_, err = NewTarget("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only target")
	}
}

func TestNewTarget_UPPERAlwaysAdded(t *testing.T) {
	// alvo curto ("v2") ainda gera UPPER matcher porque UPPER != raw.
	tg, err := NewTarget("v2")
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	if len(tg.Matchers) != 3 {
		t.Errorf("expected 3 matchers (literal + subpath + UPPER); got %d", len(tg.Matchers))
	}
}

func TestTargetBase(t *testing.T) {
	tg, _ := NewTarget("tools/cmd/memory-mcp")
	if got := tg.TargetBase(); got != "memory-mcp" {
		t.Errorf("TargetBase = %q; want %q", got, "memory-mcp")
	}
}

func TestLiteralHits_FindAndDedupe(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), "// references tools/cmd/memory-mcp\npackage a\n")
	mustWrite(t, filepath.Join(root, "b.md"), "# memory-mcp notes\nTools/Cmd/Memory-Mcp is fine\n")
	mustWrite(t, filepath.Join(root, "c.txt"), "no refs here\njust plain text\n")
	mustWrite(t, filepath.Join(root, "d.env"), "TOOLS_CMD_MEMORY_MCP_TABLE=memories\n")

	tg, _ := NewTarget("tools/cmd/memory-mcp")
	paths := []string{"a.go", "b.md", "c.txt", "d.env"}
	hits, err := LiteralHits(root, paths, tg, DefaultMaxFileBytes)
	if err != nil {
		t.Fatalf("LiteralHits: %v", err)
	}

	// Espera matches em a.go, b.md, d.env; nada em c.txt
	files := uniqueFiles(hits)
	sort.Strings(files)
	want := []string{"a.go", "b.md", "d.env"}
	if !equalStrings(files, want) {
		t.Errorf("files hit = %v; want %v", files, want)
	}

	// a.go deve ter 1 hit no matcher literal (linha 1)
	if !hasHit(hits, "a.go", 1, "tools/cmd/memory-mcp") {
		t.Errorf("expected hit a.go:1 matching tools/cmd/memory-mcp")
	}
}

func TestLiteralHits_IgnoresMissingFiles(t *testing.T) {
	root := t.TempDir()
	tg, _ := NewTarget("foo")
	hits, err := LiteralHits(root, []string{"nonexistent.go"}, tg, 1024)
	if err != nil {
		t.Fatalf("LiteralHits: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for missing file; got %d", len(hits))
	}
}

func TestLiteralHits_TruncatesLongMatch(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("X", 200)
	mustWrite(t, filepath.Join(root, "long.txt"), long+"\n")
	tg := AuditTarget{
		Raw: "XXX",
		Matchers: []TargetMatcher{
			{Label: "XXX", Regex: regexp.MustCompile("X+")},
		},
	}
	hits, err := LiteralHits(root, []string{"long.txt"}, tg, 1024)
	if err != nil {
		t.Fatalf("LiteralHits: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	if len(hits[0].Match) > 120 {
		t.Errorf("Match length %d > 120; redact should truncate", len(hits[0].Match))
	}
	if !strings.HasSuffix(hits[0].Match, "...") {
		t.Errorf("Match should end with ...; got %q", hits[0].Match)
	}
}

func TestWalk_BasicAndGitignore(t *testing.T) {
	root := t.TempDir()
	// Cria um repo git de verdade para que `git ls-files` funcione.
	mustRunCmd(t, root, "git", "init", "-q")
	mustRunCmd(t, root, "git", "config", "user.email", "test@test")
	mustRunCmd(t, root, "git", "config", "user.name", "Test")
	mustWrite(t, filepath.Join(root, "a.go"), "package a\n")
	mustWrite(t, filepath.Join(root, "b.md"), "# b\n")
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustWrite(t, filepath.Join(root, "node_modules", "skip.js"), "// skip\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "node_modules/\nb.md\n")
	mustRunCmd(t, root, "git", "add", "-A")
	mustRunCmd(t, root, "git", "commit", "-q", "-m", "init")

	paths, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if contains(paths, "node_modules/skip.js") {
		t.Errorf("Walk returned ignored node_modules file: %v", paths)
	}
	if contains(paths, "b.md") {
		t.Errorf("Walk returned gitignored b.md: %v", paths)
	}
	if !contains(paths, "a.go") {
		t.Errorf("Walk missing a.go: %v", paths)
	}
}

func TestWalk_FallbackSkipsNoisyDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "keep.go"), "package k\n")
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustWrite(t, filepath.Join(root, "node_modules", "x.js"), "// x\n")

	paths, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if contains(paths, "node_modules/x.js") {
		t.Errorf("fallback Walk should skip node_modules: %v", paths)
	}
	if !contains(paths, "keep.go") {
		t.Errorf("fallback Walk missing keep.go: %v", paths)
	}
}

// --- helpers ---

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustRunCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func uniqueFiles(hits []FileHit) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, h := range hits {
		if _, ok := seen[h.Path]; ok {
			continue
		}
		seen[h.Path] = struct{}{}
		out = append(out, h.Path)
	}
	return out
}

func hasHit(hits []FileHit, path string, line int, match string) bool {
	for _, h := range hits {
		if h.Path == path && h.Line == line && h.Match == match {
			return true
		}
	}
	return false
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

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
