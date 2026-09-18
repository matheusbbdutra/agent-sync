package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
