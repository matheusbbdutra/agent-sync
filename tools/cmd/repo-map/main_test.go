package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captureRun(t *testing.T, args []string) (stdout, stderr string, exit int) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	done := make(chan struct{})
	var outBuf, errBuf bytes.Buffer
	go func() {
		_, _ = io.Copy(&outBuf, rOut)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(&errBuf, rErr)
		done <- struct{}{}
	}()

	exit = runMain(args)

	wOut.Close()
	wErr.Close()
	<-done
	<-done

	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return outBuf.String(), errBuf.String(), exit
}

func TestRunUpdateEmDirTemporario(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update", "--quiet"})
	if exit != 0 {
		t.Errorf("esperava exit 0, obteve %d (stdout=%q)", exit, stdout)
	}
	if stdout != "" {
		t.Errorf("--quiet deveria suprimir stdout, obteve %q", stdout)
	}

	stdout, _, exit = captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update"})
	if exit != 0 {
		t.Errorf("segundo update exit=%d", exit)
	}
	if !strings.Contains(stdout, "reusados") {
		t.Errorf("esperava mencionar 'reusados' em stdout, obteve %q", stdout)
	}
}

func TestRunFocusSemCacheRetornaAviso(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	_, stderr, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--focus", "main.go"})
	if exit != 1 {
		t.Errorf("esperava exit 1 sem cache, obteve %d", exit)
	}
	if !strings.Contains(stderr, "execute repo-map --update") {
		t.Errorf("esperava mensagem sobre --update, obteve %q", stderr)
	}
}

func TestRunSemFlags(t *testing.T) {
	_, stderr, exit := captureRun(t, []string{})
	if exit != 2 {
		t.Errorf("esperava exit 2 sem modo, obteve %d", exit)
	}
	if !strings.Contains(stderr, "nenhum modo informado") {
		t.Errorf("esperava erro 'nenhum modo informado', obteve %q", stderr)
	}
}

func TestRunVersion(t *testing.T) {
	stdout, _, exit := captureRun(t, []string{"--version"})
	if exit != 0 {
		t.Errorf("esperava exit 0, obteve %d", exit)
	}
	if !strings.Contains(stdout, "v1") {
		t.Errorf("esperava mencionar versão, obteve %q", stdout)
	}
}

func TestRunFocusAposUpdate(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lib.go"), []byte("package lib\nfunc Run(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nimport \"lib\"\nfunc main(){ lib.Run() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update", "--quiet"}); exit != 0 {
		t.Fatal("update inicial falhou")
	}

	stdout, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--focus", "lib.go"})
	if exit != 0 {
		t.Errorf("focus exit=%d", exit)
	}
	if !strings.Contains(stdout, "lib.go") {
		t.Errorf("focus deveria mencionar lib.go, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "main.go") {
		t.Errorf("focus deveria listar main.go como importer, stdout=%q", stdout)
	}
}

func TestRunSummaryAposUpdate(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update", "--quiet"}); exit != 0 {
		t.Fatal("update inicial falhou")
	}

	stdout, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--summary"})
	if exit != 0 {
		t.Errorf("summary exit=%d", exit)
	}
	if !strings.Contains(stdout, "Top hubs") {
		t.Errorf("summary deveria começar com 'Top hubs', stdout=%q", stdout)
	}
}

func TestRunSummaryLimitaPorMaxTokens(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update", "--quiet"}); exit != 0 {
		t.Fatal("update inicial falhou")
	}

	stdout, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--summary", "--max-tokens", "5"})
	if exit != 0 {
		t.Errorf("summary exit=%d", exit)
	}
	if len(strings.Fields(stdout)) > 50 {
		t.Errorf("summary com max-tokens=5 deveria ser curto, stdout=%q", stdout)
	}
}

func TestRunBrief(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, ".agent-sync", "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "service.go"), []byte("package svc\nimport \"os\"\nfunc Run() { _ = os.Getenv(\"PORT\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--update", "--quiet"}); exit != 0 {
		t.Fatal("update inicial falhou")
	}

	stdout, _, exit := captureRun(t, []string{"--root", dir, "--cache-dir", cache, "--brief", "service.go"})
	if exit != 0 {
		t.Errorf("brief exit=%d", exit)
	}
	if !strings.Contains(stdout, `"blast_radius":`) {
		t.Errorf("brief deveria conter blast_radius, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, `"PORT"`) {
		t.Errorf("brief deveria conter env PORT, stdout=%q", stdout)
	}
}

func TestRunAuditRemoval(t *testing.T) {
	dir := t.TempDir()
	// arquivo referenciado + schema inline
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# tools/cmd/foo\nreference\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deps.go"), []byte("package x\nimport \"tools/cmd/foo/store\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.go"), []byte("package s\nconst ddl = `CREATE TABLE foo (id INTEGER)`\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, exit := captureRun(t, []string{"--root", dir, "--audit-removal", "tools/cmd/foo"})
	if exit != 0 {
		t.Errorf("audit-removal exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stdout, `"schema_version": "agent-sync.audit-ledger.v1"`) {
		t.Errorf("output deveria ter schema_version, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, `"tools-cmd-foo-removal"`) {
		t.Errorf("output deveria ter id slug, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, `"go-package-reference"`) {
		t.Errorf("output deveria ter classe go-package-reference, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, `"foo"`) {
		t.Errorf("output deveria mencionar tabela foo, stdout=%q", stdout)
	}
}

func TestRunAuditRemovalLatencia(t *testing.T) {
	// gera ~200 arquivos pequenos para validar que o walk+grep fica abaixo
	// do orçamento de 500ms definido no ADR A-73 (Critério #2).
	dir := t.TempDir()
	for i := 0; i < 200; i++ {
		name := filepath.Join(dir, "file"+itoaPad(i, 3)+".md")
		if err := os.WriteFile(name, []byte("# target-name notes\nsome content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	stdout, stderr, exit := captureRun(t, []string{"--root", dir, "--audit-removal", "target-name"})
	elapsed := time.Since(start)
	if exit != 0 {
		t.Errorf("audit-removal exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stdout, `"target-name-removal"`) {
		t.Errorf("output deveria conter id, stdout=%q", stdout)
	}
	t.Logf("latência para 200 arquivos: %v", elapsed)
	if elapsed > 500*time.Millisecond {
		t.Errorf("latência %v excedeu orçamento de 500ms (ADR A-73)", elapsed)
	}
}

func itoaPad(n, width int) string {
	s := []byte{}
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	for len(s) < width {
		s = append([]byte{'0'}, s...)
	}
	if len(s) == 0 {
		return "0"
	}
	return string(s)
}

