package evalrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/evalsuite"
)

// Runner com comando externo: o caso vai em stdin e o stdout e a saida avaliada.
func TestExecRunnerDevolveStdout(t *testing.T) {
	r := ExecRunner{Command: "cat"}
	got, err := r.Run(evalsuite.Case{ID: "c1", Slice: "s"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(got, `"id":"c1"`) {
		t.Errorf("stdout nao contem o caso enviado em stdin: %q", got)
	}
}

// O comando pode transformar o payload: aqui devolve so o id do caso.
func TestExecRunnerComandoArbitrario(t *testing.T) {
	r := ExecRunner{Command: "sed 's/.*\"id\":\"\\([^\"]*\\)\".*/\\1/'"}
	got, err := r.Run(evalsuite.Case{ID: "abc-123", Slice: "s"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(got) != "abc-123" {
		t.Errorf("saida = %q, quer \"abc-123\"", strings.TrimSpace(got))
	}
}

// Exit != 0 e erro de infraestrutura, nunca veredito: o caller aborta.
func TestExecRunnerExitNaoZeroEerro(t *testing.T) {
	r := ExecRunner{Command: "exit 3"}
	if _, err := r.Run(evalsuite.Case{ID: "c1", Slice: "s"}); err == nil {
		t.Fatal("esperava erro para exit != 0, obteve nil")
	}
}

// Erro do runner nao pode virar pass: aqui o comando falha e o caso nao e avaliado.
func TestExecRunnerStderrVaiNaMensagem(t *testing.T) {
	r := ExecRunner{Command: "echo 'boom' >&2; exit 1"}
	_, err := r.Run(evalsuite.Case{ID: "c1", Slice: "s"})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("erro deveria citar stderr, obteve: %v", err)
	}
}

// Arquivo temporario como runner: cobre o caminho de execucao por arquivo.
func TestExecRunnerScriptDeArquivo(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "runner.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'saida-do-runner'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := ExecRunner{Command: script}
	got, err := r.Run(evalsuite.Case{ID: "c1", Slice: "s"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "saida-do-runner" {
		t.Errorf("saida = %q", got)
	}
}
