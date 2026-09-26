package audit

import "testing"

func TestClassifyHit_DocsActive(t *testing.T) {
	tg, _ := NewTarget("tools/cmd/memory-mcp")
	hit := FileHit{Path: "README.md", Line: 5, LineText: "uses memory-mcp internally"}
	if got := ClassifyHit(hit, tg, nil); got != ClassDocsActive {
		t.Errorf("ClassifyHit(.md) = %v; want %v", got, ClassDocsActive)
	}
}

func TestClassifyHit_DocsHistorical(t *testing.T) {
	tg, _ := NewTarget("oldapi")
	hit := FileHit{Path: "docs/archive/oldapi.md", Line: 1, LineText: "oldapi history"}
	if got := ClassifyHit(hit, tg, nil); got != ClassDocsHistorical {
		t.Errorf("ClassifyHit(archive/) = %v; want %v", got, ClassDocsHistorical)
	}
}

func TestClassifyHit_PackageJSON(t *testing.T) {
	tg, _ := NewTarget("foo")
	hit := FileHit{Path: "package.json", Line: 12, LineText: `"foo": "^1.0.0"`}
	if got := ClassifyHit(hit, tg, nil); got != ClassPackageDependency {
		t.Errorf("ClassifyHit(package.json) = %v; want %v", got, ClassPackageDependency)
	}
}

func TestClassifyHit_Lockfile(t *testing.T) {
	tg, _ := NewTarget("foo")
	cases := []struct {
		path string
		want EvidenceClass
	}{
		{"bun.lock", ClassLockfileReference},
		{"package-lock.json", ClassLockfileReference},
		{"go.sum", ClassLockfileReference},
		{"yarn.lock", ClassLockfileReference},
	}
	for _, c := range cases {
		hit := FileHit{Path: c.path, Line: 1, LineText: "any foo ref"}
		if got := ClassifyHit(hit, tg, nil); got != c.want {
			t.Errorf("ClassifyHit(%s) = %v; want %v", c.path, got, c.want)
		}
	}
}

func TestClassifyHit_EnvVar(t *testing.T) {
	tg, _ := NewTarget("memory-mcp")
	hit := FileHit{Path: "scripts/run.sh", Line: 3, LineText: "MEMORY_MCP_TOKEN=abc"}
	if got := ClassifyHit(hit, tg, nil); got != ClassEnvVar {
		t.Errorf("ClassifyHit(env-var) = %v; want %v", got, ClassEnvVar)
	}

	// também detecta em .env
	hit = FileHit{Path: ".env", Line: 1, LineText: "FOO=bar"}
	if got := ClassifyHit(hit, tg, nil); got != ClassEnvVar {
		t.Errorf("ClassifyHit(.env) = %v; want %v", got, ClassEnvVar)
	}
}

func TestClassifyHit_ImportOrSDKClient(t *testing.T) {
	tg, _ := NewTarget("foo")
	cases := []string{
		`import { foo } from "bar"`,
		`const x = require("foo")`,
		`from foo import bar`,
		`use Foo\Bar;`,
	}
	for _, line := range cases {
		hit := FileHit{Path: "src/main.ts", Line: 1, LineText: line}
		if got := ClassifyHit(hit, tg, nil); got != ClassImportOrSDKClient {
			t.Errorf("ClassifyHit(%q) = %v; want %v", line, got, ClassImportOrSDKClient)
		}
	}
}

func TestClassifyHit_GoPackageReference(t *testing.T) {
	tg, _ := NewTarget("tools/cmd/memory-mcp")
	cases := []struct {
		path, line string
		want       bool
	}{
		{"internal/memory-mcp/command.go", "// ref to memory-mcp", true},
		{"foo/memory_mcp.go", "package x", false}, // underscore, não casa
		{"foo/memory-mcp.go", "package x", true},
		{"src/imports.go", `import "github.com/x/tools/cmd/memory-mcp/store"`, true},
		{"other/pkg/file.go", "// mentions memory-mcp only in comment", false},
	}
	for _, c := range cases {
		hit := FileHit{Path: c.path, Line: 1, LineText: c.line}
		got := ClassifyHit(hit, tg, nil)
		if (got == ClassGoPackageReference) != c.want {
			t.Errorf("ClassifyHit(%q,%q) = %v; wantGo=%v", c.path, c.line, got, c.want)
		}
	}
}

func TestClassifyHit_SQLTableReference(t *testing.T) {
	tg, _ := NewTarget("foo")
	// .sql sempre conta
	hit := FileHit{Path: "migrations/001.sql", Line: 5, LineText: "CREATE TABLE foo (...)"}
	if got := ClassifyHit(hit, tg, nil); got != ClassSQLTableReference {
		t.Errorf("ClassifyHit(.sql) = %v; want %v", got, ClassSQLTableReference)
	}

	// .go só conta se tabela aparecer na linha E estiver no schema
	hit = FileHit{Path: "internal/store/store.go", Line: 1, LineText: `db.Query("SELECT * FROM memories")`}
	if got := ClassifyHit(hit, tg, []string{"memories", "events"}); got != ClassSQLTableReference {
		t.Errorf("ClassifyHit(.go com tabela) = %v; want %v", got, ClassSQLTableReference)
	}

	// .go sem match de tabela não classifica como SQL
	hit = FileHit{Path: "internal/foo/main.go", Line: 1, LineText: "// no table here"}
	if got := ClassifyHit(hit, tg, []string{"memories"}); got == ClassSQLTableReference {
		t.Errorf("ClassifyHit(.go sem tabela) unexpectedly = %v", got)
	}
}

func TestClassifyHit_UnknownLiteralHit(t *testing.T) {
	tg, _ := NewTarget("bar")
	hit := FileHit{Path: "some/random/file.bin", Line: 1, LineText: "bar"}
	// arquivo binário não tem extensão classificada → default
	if got := ClassifyHit(hit, tg, nil); got != ClassUnknownLiteralHit {
		t.Errorf("ClassifyHit(binário) = %v; want %v", got, ClassUnknownLiteralHit)
	}
}

func TestAllClasses_Order(t *testing.T) {
	all := AllClasses()
	if len(all) != 9 {
		t.Errorf("expected 9 classes; got %d", len(all))
	}
	seen := map[EvidenceClass]bool{}
	for _, c := range all {
		if seen[c] {
			t.Errorf("classe duplicada: %v", c)
		}
		seen[c] = true
	}
}
