package main

import (
	"strings"
	"testing"
)

func TestParseDiffModifiedWithMultipleHunks(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
index 111..222 100644
--- a/foo.go
+++ b/foo.go
@@ -1,4 +1,5 @@ func main() {
 package main
-import "old"
+import "new"
+import "extra"
 
@@ -10,2 +11,2 @@ func helper() {
-	return 1
+	return 2
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}

	file := files[0]
	if file.Path != "foo.go" {
		t.Fatalf("caminho errado: %q", file.Path)
	}
	if file.Status != statusModified {
		t.Fatalf("status errado: %q", file.Status)
	}
	if len(file.Hunks) != 2 {
		t.Fatalf("esperava 2 hunks, got %d", len(file.Hunks))
	}

	first := file.Hunks[0]
	if first.Header != "func main() {" {
		t.Fatalf("cabeçalho do 1º hunk errado: %q", first.Header)
	}
	if first.Added != 2 || first.Removed != 1 {
		t.Fatalf("contagem do 1º hunk errada: +%d/-%d", first.Added, first.Removed)
	}

	second := file.Hunks[1]
	if second.Header != "func helper() {" {
		t.Fatalf("cabeçalho do 2º hunk errado: %q", second.Header)
	}
	if second.Added != 1 || second.Removed != 1 {
		t.Fatalf("contagem do 2º hunk errada: +%d/-%d", second.Added, second.Removed)
	}
}

func TestParseDiffDetectsAddedFile(t *testing.T) {
	diff := `diff --git a/new.go b/new.go
new file mode 100644
index 000..123
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+line1
+line2
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}
	if files[0].Status != statusAdded {
		t.Fatalf("status errado: %q", files[0].Status)
	}
	if files[0].Path != "new.go" {
		t.Fatalf("caminho errado: %q", files[0].Path)
	}
	if files[0].Hunks[0].Added != 2 || files[0].Hunks[0].Removed != 0 {
		t.Fatalf("contagem errada: +%d/-%d", files[0].Hunks[0].Added, files[0].Hunks[0].Removed)
	}
}

func TestParseDiffDetectsDeletedFile(t *testing.T) {
	diff := `diff --git a/old.go b/old.go
deleted file mode 100644
index 123..000
--- a/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-line1
-line2
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}
	if files[0].Status != statusDeleted {
		t.Fatalf("status errado: %q", files[0].Status)
	}
	if files[0].Path != "old.go" {
		t.Fatalf("caminho errado: %q", files[0].Path)
	}
	if files[0].Hunks[0].Removed != 2 || files[0].Hunks[0].Added != 0 {
		t.Fatalf("contagem errada: +%d/-%d", files[0].Hunks[0].Added, files[0].Hunks[0].Removed)
	}
}

func TestParseDiffPlainUnifiedMultipleFiles(t *testing.T) {
	diff := "--- old/a.txt\t2024-01-01 10:00:00\n" +
		"+++ new/a.txt\t2024-01-01 10:00:00\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n" +
		"--- old/b.txt\t2024-01-01 10:00:00\n" +
		"+++ new/b.txt\t2024-01-01 10:00:00\n" +
		"@@ -1 +1 @@ func b()\n" +
		"-x\n" +
		"+y\n"

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("esperava 2 arquivos, got %d: %+v", len(files), files)
	}
	if files[0].Path != "new/a.txt" || files[1].Path != "new/b.txt" {
		t.Fatalf("caminhos errados: %q, %q", files[0].Path, files[1].Path)
	}
	if files[1].Hunks[0].Header != "func b()" {
		t.Fatalf("cabeçalho errado: %q", files[1].Hunks[0].Header)
	}
}

func TestParseDiffHunkWithoutHeader(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,2 +1,2 @@
-old
+new
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := files[0].Hunks[0].Header; got != "" {
		t.Fatalf("esperava cabeçalho vazio, got %q", got)
	}
}

func TestParseDiffTreatsHeaderLookalikesAsContent(t *testing.T) {
	diff := `diff --git a/x b/x
--- a/x
+++ b/x
@@ -1,2 +1,2 @@
--- foo
+++ bar
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}
	hunk := files[0].Hunks[0]
	if hunk.Added != 1 || hunk.Removed != 1 {
		t.Fatalf("contagem errada: +%d/-%d", hunk.Added, hunk.Removed)
	}
}

func TestParseDiffNoHunks(t *testing.T) {
	diff := `diff --git a/empty.go b/empty.go
old mode 100644
new mode 100755
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}
	if files[0].Path != "empty.go" {
		t.Fatalf("caminho errado: %q", files[0].Path)
	}
	if len(files[0].Hunks) != 0 {
		t.Fatalf("esperava nenhum hunk, got %d", len(files[0].Hunks))
	}
}

func TestParseDiffMnemonicPrefixes(t *testing.T) {
	diff := `diff --git i/foo.go w/foo.go
index 111..222 100644
--- i/foo.go
+++ w/foo.go
@@ -1 +1 @@
-old
+new
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("esperava 1 arquivo, got %d", len(files))
	}
	if files[0].Path != "foo.go" {
		t.Fatalf("caminho errado: %q", files[0].Path)
	}
}

func TestParseDiffNoPrefixConfig(t *testing.T) {
	diff := `diff --git foo.go foo.go
index 111..222 100644
--- foo.go
+++ foo.go
@@ -1 +1 @@
-old
+new
`

	files, err := ParseDiff(strings.NewReader(diff))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if files[0].Path != "foo.go" {
		t.Fatalf("caminho errado: %q", files[0].Path)
	}
}

func TestFormatSummary(t *testing.T) {
	files := []FileSummary{
		{
			Path:   "foo.go",
			Status: statusModified,
			Hunks: []HunkSummary{
				{Header: "func main() {", Added: 2, Removed: 1},
				{Header: "", Added: 1, Removed: 0},
			},
		},
		{
			Path:   "bar.go",
			Status: statusAdded,
		},
	}

	out := FormatSummary(files)
	want := "[M] foo.go\n" +
		"  @@ func main() { (+2/-1)\n" +
		"  @@ (sem cabeçalho) (+1/-0)\n" +
		"[A] bar.go\n" +
		"  (sem hunks)\n"

	if out != want {
		t.Fatalf("saída inesperada:\n--- got ---\n%s--- want ---\n%s", out, want)
	}
}
