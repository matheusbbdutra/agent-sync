package evalsuite

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSuite = `{"id":"a","slice":"s1","input":{"kind":"prompt","text":"x"},"expect":{"must_match":["foo"]}}
{"id":"b","slice":"s1","weight":2,"input":{"kind":"prompt","text":"y"},"expect":{"must_match":["bar"]},"judge":{"rubric":"r","max_score":5,"threshold":0.6}}
`

func TestParseValidAppliesWeightDefault(t *testing.T) {
	cases, err := Parse(strings.NewReader(validSuite))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("casos = %d, quer 2", len(cases))
	}
	if cases[0].Weight != 1 {
		t.Errorf("weight default = %v, quer 1", cases[0].Weight)
	}
	if cases[1].Weight != 2 {
		t.Errorf("weight explicito = %v, quer 2", cases[1].Weight)
	}
}

func TestParseSkipsBlankLines(t *testing.T) {
	src := "\n\n" + validSuite + "\n\n"
	if _, err := Parse(strings.NewReader(src)); err != nil {
		t.Fatalf("Parse com linhas vazias: %v", err)
	}
}

func TestParseEmptySuiteFails(t *testing.T) {
	_, err := Parse(strings.NewReader("\n  \n"))
	if err == nil {
		t.Fatal("suite sem casos deveria falhar")
	}
}

func TestParseBrokenJSONFails(t *testing.T) {
	_, err := Parse(strings.NewReader(`{"id":"a","slice":"s1"` + "\n"))
	if err == nil || !strings.Contains(err.Error(), "linha 1") {
		t.Fatalf("erro = %v, quer erro na linha 1", err)
	}
}

func TestValidateIssues(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		field string
	}{
		{
			name:  "id vazio",
			line:  `{"id":"","slice":"s1","input":{"kind":"k","text":"t"},"expect":{"must_match":["x"]}}`,
			field: "id",
		},
		{
			name:  "slice vazio",
			line:  `{"id":"a","slice":"","input":{"kind":"k","text":"t"},"expect":{"must_match":["x"]}}`,
			field: "slice",
		},
		{
			name:  "weight negativo",
			line:  `{"id":"a","slice":"s1","weight":-1,"input":{"kind":"k","text":"t"},"expect":{"must_match":["x"]}}`,
			field: "weight",
		},
		{
			name:  "sem regra nenhuma",
			line:  `{"id":"a","slice":"s1","input":{"kind":"k","text":"t"}}`,
			field: "expect",
		},
		{
			name:  "json_schema com required invalido",
			line:  `{"id":"a","slice":"s1","input":{"kind":"k","text":"t"},"expect":{"json_schema":{"required":"texto"}}}`,
			field: "expect.json_schema",
		},
		{
			name:  "judge sem rubric",
			line:  `{"id":"a","slice":"s1","input":{"kind":"k","text":"t"},"judge":{"max_score":5,"threshold":0.5}}`,
			field: "judge.rubric",
		},
		{
			name:  "judge threshold acima de 1",
			line:  `{"id":"a","slice":"s1","input":{"kind":"k","text":"t"},"judge":{"rubric":"r","max_score":5,"threshold":4}}`,
			field: "judge.threshold",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.line + "\n"))
			var inv *InvalidSuiteError
			if !errors.As(err, &inv) {
				t.Fatalf("erro = %v, quer *InvalidSuiteError", err)
			}
			if len(inv.Issues) != 1 {
				t.Fatalf("issues = %d, quer 1 (%v)", len(inv.Issues), inv.Issues)
			}
			if inv.Issues[0].Field != tc.field {
				t.Errorf("field = %q, quer %q", inv.Issues[0].Field, tc.field)
			}
		})
	}
}

func TestValidateDuplicateID(t *testing.T) {
	line := `{"id":"dup","slice":"s1","input":{"kind":"k","text":"t"},"expect":{"must_match":["x"]}}`
	_, err := Parse(strings.NewReader(line + "\n" + line + "\n"))
	var inv *InvalidSuiteError
	if !errors.As(err, &inv) {
		t.Fatalf("erro = %v, quer *InvalidSuiteError", err)
	}
	if len(inv.Issues) != 1 || inv.Issues[0].Field != "id" {
		t.Fatalf("issues = %v, quer 1 issue de id", inv.Issues)
	}
	if !strings.Contains(inv.Issues[0].Message, "duplicado") {
		t.Errorf("mensagem = %q, quer conter 'duplicado'", inv.Issues[0].Message)
	}
	if inv.Issues[0].Line != 2 {
		t.Errorf("linha = %d, quer 2", inv.Issues[0].Line)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nao-existe.jsonl")); err == nil {
		t.Fatal("arquivo inexistente deveria falhar")
	}
}

func TestLoadValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.jsonl")
	if err := os.WriteFile(path, []byte(validSuite), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cases, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("casos = %d, quer 2", len(cases))
	}
}

func TestHasRules(t *testing.T) {
	onlyJudge := Case{ID: "a", Slice: "s", Judge: &Judge{Rubric: "r", MaxScore: 5, Threshold: 0.5}}
	if !onlyJudge.HasRules() || onlyJudge.HasDeterministicRules() {
		t.Error("caso so com judge deveria ter regra e nao ser deterministico")
	}
	onlyExpect := Case{ID: "b", Slice: "s", Expect: Expect{MustMatch: []string{"x"}}}
	if !onlyExpect.HasRules() || !onlyExpect.HasDeterministicRules() {
		t.Error("caso com must_match deveria ser deterministico")
	}
	if (Case{ID: "c", Slice: "s"}).HasRules() {
		t.Error("caso sem regra nao deveria ter regra")
	}
}

func TestRequiredFields(t *testing.T) {
	c := Case{ID: "a", Slice: "s", Expect: Expect{JSONSchema: []byte(`{"type":"object","required":["nome","idade"]}`)}}
	got := c.RequiredFields()
	if len(got) != 2 || got[0] != "nome" || got[1] != "idade" {
		t.Fatalf("required = %v, quer [nome idade]", got)
	}
	if (Case{}).RequiredFields() != nil {
		t.Error("caso sem json_schema deveria devolver nil")
	}
}

// A suite de smoke do binario precisa passar na validacao: e o contrato que o
// subcomando `eval-run suite check` usa na pratica.
func TestSmokeSuiteIsValid(t *testing.T) {
	cases, err := Load(filepath.Join("..", "..", "cmd", "eval-run", "testdata", "smoke.jsonl"))
	if err != nil {
		t.Fatalf("smoke.jsonl invalido: %v", err)
	}
	if len(cases) != 10 {
		t.Fatalf("casos = %d, quer 10", len(cases))
	}
}
