package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runner executa runMain com buffers limpos e devolve exit code + saidas.
func runner(args []string, stdin string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runMain(args, &stdout, &stderr, strings.NewReader(stdin))
	return code, stdout.String(), stderr.String()
}

// writePrompt grava o prompt em um arquivo temporario e devolve o caminho.
func writePrompt(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("gravar prompt: %v", err)
	}
	return path
}

func TestRunMainExitCodes(t *testing.T) {
	clean := writePrompt(t, "Responda em JSON com os campos id e nome.\n")
	negation := writePrompt(t, "Nunca invente nome de funcao.\n")

	cases := []struct {
		name string
		args []string
		want int
	}{
		{name: "sem argumento", args: nil, want: exitUsage},
		{name: "subcomando desconhecido", args: []string{"lint"}, want: exitUsage},
		{name: "help", args: []string{"--help"}, want: exitOK},
		{name: "sem caminho", args: []string{"check"}, want: exitUsage},
		{name: "arquivo inexistente", args: []string{"check", filepath.Join(t.TempDir(), "nao-existe.md")}, want: exitUsage},
		{name: "severidade invalida", args: []string{"check", clean, "--severity", "info"}, want: exitUsage},
		{name: "prompt limpo", args: []string{"check", clean}, want: exitOK},
		{name: "prompt com achado", args: []string{"check", negation}, want: exitFindings},
		{name: "achado abaixo do limiar error", args: []string{"check", negation, "--severity", "error"}, want: exitOK},
		{name: "flags antes do caminho", args: []string{"check", "--severity", "error", negation}, want: exitOK},
		{name: "stdin com achado", args: []string{"check", "-"}, want: exitFindings},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := runner(tc.args, "Nunca invente nome de funcao.\n")
			if code != tc.want {
				t.Fatalf("exit = %d, quer %d", code, tc.want)
			}
		})
	}
}

func TestCheckJSON(t *testing.T) {
	path := writePrompt(t, "Nunca invente nome de funcao.\nContexto: {{cliente}}\n")
	code, stdout, stderr := runner([]string{"check", path, "--json"}, "")
	if code != exitFindings {
		t.Fatalf("exit = %d, quer %d (stderr=%s)", code, exitFindings, stderr)
	}

	var report struct {
		Prompt   string `json:"prompt"`
		Chars    int    `json:"chars"`
		Lines    int    `json:"lines"`
		Severity string `json:"severity"`
		Summary  struct {
			Total    int            `json:"total"`
			Errors   int            `json:"errors"`
			Warnings int            `json:"warnings"`
			ByCheck  map[string]int `json:"by_check"`
		} `json:"summary"`
		Findings []struct {
			Check    string `json:"check"`
			Severity string `json:"severity"`
			Line     int    `json:"line"`
			Message  string `json:"message"`
			Fix      string `json:"fix"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("json invalido: %v\n%s", err, stdout)
	}
	if report.Prompt != path || report.Chars == 0 || report.Lines != 2 {
		t.Errorf("cabecalho json = %+v", report)
	}
	if report.Severity != "warning" {
		t.Errorf("severity = %q, quer warning", report.Severity)
	}
	if len(report.Findings) != 2 || report.Summary.Total != 2 {
		t.Fatalf("findings/total = %d/%d, quer 2/2:\n%s", len(report.Findings), report.Summary.Total, stdout)
	}
	if report.Findings[0].Check != "negation" || report.Findings[0].Line != 1 {
		t.Errorf("primeiro finding = %+v", report.Findings[0])
	}
	if report.Findings[0].Fix == "" || report.Findings[0].Message == "" {
		t.Errorf("finding sem mensagem/fix: %+v", report.Findings[0])
	}
	if report.Summary.Errors != 1 || report.Summary.Warnings != 1 {
		t.Errorf("contadores = %+v", report.Summary)
	}
	if len(report.Summary.ByCheck) != 5 {
		t.Errorf("by_check tem %d chaves, quer 5: %+v", len(report.Summary.ByCheck), report.Summary.ByCheck)
	}
}

func TestCheckJSONSemAchadoUsaListaVazia(t *testing.T) {
	path := writePrompt(t, "Responda em JSON com os campos id e nome.\n")
	code, stdout, _ := runner([]string{"check", path, "--json"}, "")
	if code != exitOK {
		t.Fatalf("exit = %d, quer %d", code, exitOK)
	}
	if !strings.Contains(stdout, `"findings": []`) {
		t.Errorf("findings vazio deveria serializar como []:\n%s", stdout)
	}
}

func TestCheckQuietOmiteContagem(t *testing.T) {
	path := writePrompt(t, "Nunca invente nome de funcao.\n")
	code, stdout, _ := runner([]string{"check", path, "--quiet"}, "")
	if code != exitFindings {
		t.Fatalf("exit = %d, quer %d", code, exitFindings)
	}
	if strings.Contains(stdout, "contagem por check") {
		t.Errorf("--quiet ainda imprimiu contagem:\n%s", stdout)
	}
	if !strings.Contains(stdout, "negation") {
		t.Errorf("--quiet deveria imprimir a linha do achado:\n%s", stdout)
	}
}

func TestCheckTextoSempreImprimeContagem(t *testing.T) {
	path := writePrompt(t, "Nunca invente nome de funcao.\n")
	_, stdout, _ := runner([]string{"check", path}, "")
	for _, want := range []string{"contagem por check:", "negation", "no-terminator", "undeclared-output-format", "non-disjoint-classes", "placeholder-leak"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("saida sem %q:\n%s", want, stdout)
		}
	}
}

func TestCheckStdinUsaRodapeStdin(t *testing.T) {
	code, stdout, _ := runner([]string{"check", "-", "--json"}, "Nunca invente nome de funcao.\n")
	if code != exitFindings {
		t.Fatalf("exit = %d, quer %d", code, exitFindings)
	}
	var report struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("json invalido: %v\n%s", err, stdout)
	}
	if report.Prompt != "<stdin>" {
		t.Errorf("prompt = %q, quer <stdin>", report.Prompt)
	}
}

// TestTestdata cobre os 2 prompts de referencia do pacote.
func TestTestdata(t *testing.T) {
	code, stdout, stderr := runner([]string{"check", "testdata/defeituoso.md", "--json"}, "")
	if code != exitFindings {
		t.Fatalf("defeituoso.md exit = %d, quer %d (stderr=%s)", code, exitFindings, stderr)
	}
	checks := map[string]bool{}
	for _, check := range []string{"negation", "no-terminator", "undeclared-output-format", "non-disjoint-classes", "placeholder-leak"} {
		if strings.Contains(stdout, `"check": "`+check+`"`) {
			checks[check] = true
		}
	}
	if len(checks) < 3 {
		t.Fatalf("defeituoso.md disparou %d checks, quer 3+: %v\n%s", len(checks), checks, stdout)
	}

	code, stdout, _ = runner([]string{"check", "testdata/limpo.md", "--json"}, "")
	if code != exitOK {
		t.Fatalf("limpo.md exit = %d, quer %d:\n%s", code, exitOK, stdout)
	}
	if !strings.Contains(stdout, `"total": 0`) {
		t.Fatalf("limpo.md disparou achado:\n%s", stdout)
	}
}
