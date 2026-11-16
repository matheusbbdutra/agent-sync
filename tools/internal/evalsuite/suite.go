// Package evalsuite carrega e valida suites de avaliacao em JSONL.
//
// Uma suite e um arquivo de texto com um caso JSON por linha (linhas vazias
// ignoradas). O schema de cada caso:
//
//	{"id":"...","slice":"...","weight":1,
//	 "input":{"kind":"...","text":"..."},
//	 "expect":{"must_match":["..."],"must_not_match":["..."],"json_schema":{...}},
//	 "judge":{"rubric":"...","max_score":5,"threshold":0.6}}
//
// O pacote nao pontua nada: apenas garante que a suite e comparavel entre
// execucoes. Pontuacao deterministica vive em internal/evalrun.
package evalsuite

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	jsonschemav6 "github.com/santhosh-tekuri/jsonschema/v6"
)

// Case e um caso de avaliacao. Weight ausente vira 1 no parse (ver Parse).
type Case struct {
	ID     string  `json:"id"`
	Slice  string  `json:"slice"`
	Weight float64 `json:"weight"`
	Input  Input   `json:"input"`
	Expect Expect  `json:"expect"`
	Judge  *Judge  `json:"judge,omitempty"`

	line int // numero da linha no JSONL, usado nas mensagens de validacao
}

// Input descreve o estimulo do caso (ex.: o pedido do usuario).
type Input struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Expect agrupa as regras deterministicas. Qualquer subconjunto pode ser
// usado; a ausencia total de regras deterministicas exige bloco judge.
type Expect struct {
	MustMatch    []string        `json:"must_match,omitempty"`
	MustNotMatch []string        `json:"must_not_match,omitempty"`
	JSONSchema   json.RawMessage `json:"json_schema,omitempty"`
}

// Judge descreve a regra de nota por rubric (nao deterministica). O score
// devolvido pelo juiz e normalizado por MaxScore e comparado com Threshold.
type Judge struct {
	Rubric    string  `json:"rubric"`
	MaxScore  float64 `json:"max_score"`
	Threshold float64 `json:"threshold"`
}

// HasDeterministicRules indica se o caso tem ao menos uma regra em expect.
func (c Case) HasDeterministicRules() bool {
	return len(c.Expect.MustMatch) > 0 || len(c.Expect.MustNotMatch) > 0 || len(c.Expect.JSONSchema) > 0
}

// HasRules indica se o caso tem ao menos uma regra pontuavel (expect ou judge).
func (c Case) HasRules() bool { return c.HasDeterministicRules() || c.Judge != nil }

// Issue e um problema de validacao localizado em um caso.
type Issue struct {
	Line    int    `json:"line"`
	CaseID  string `json:"case_id,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	where := fmt.Sprintf("linha %d", i.Line)
	if i.CaseID != "" {
		where += " (id=" + i.CaseID + ")"
	}
	return fmt.Sprintf("%s [%s] %s", where, i.Field, i.Message)
}

// InvalidSuiteError agrega todas as issues de uma suite invalida.
type InvalidSuiteError struct{ Issues []Issue }

func (e *InvalidSuiteError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, iss := range e.Issues {
		parts = append(parts, iss.String())
	}
	return fmt.Sprintf("suite invalida (%d problema(s)):\n  %s", len(e.Issues), strings.Join(parts, "\n  "))
}

// Parse le o JSONL e devolve os casos com weight default aplicado. Erros de
// sintaxe JSON abortam o parse (nao ha como numerar o restante), mas erros de
// validacao sao agregados em *InvalidSuiteError.
func Parse(r io.Reader) ([]Case, error) {
	var cases []Case
	var issues []Issue
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var c Case
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return nil, fmt.Errorf("linha %d: JSON invalido: %w", line, err)
		}
		c.line = line
		if c.Weight == 0 {
			c.Weight = 1
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ler JSONL: %w", err)
	}
	if len(cases) == 0 {
		return nil, errors.New("suite vazia: nenhum caso no arquivo")
	}
	issues = append(issues, Validate(cases)...)
	if len(issues) > 0 {
		return nil, &InvalidSuiteError{Issues: issues}
	}
	return cases, nil
}

// Load le o arquivo, parseia e valida a suite.
func Load(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cases, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cases, nil
}

// Validate devolve todas as issues dos casos: id unico e nao vazio, slice nao
// vazio, weight > 0, ao menos uma regra (expect ou judge) e json_schema
// compilavel. O bloco judge, quando presente, precisa de rubric, max_score > 0
// e 0 < threshold <= 1 (threshold e fracao de max_score, nao pontuacao).
func Validate(cases []Case) []Issue {
	var issues []Issue
	seen := map[string]int{}
	for _, c := range cases {
		add := func(field, msg string) {
			issues = append(issues, Issue{Line: c.line, CaseID: c.ID, Field: field, Message: msg})
		}
		if c.ID == "" {
			add("id", "id vazio")
		} else if prev, dup := seen[c.ID]; dup {
			add("id", fmt.Sprintf("id duplicado (ja usado na linha %d)", prev))
		} else {
			seen[c.ID] = c.line
		}
		if c.Slice == "" {
			add("slice", "slice vazio")
		}
		if c.Weight <= 0 {
			add("weight", fmt.Sprintf("weight deve ser > 0 (recebido %v)", c.Weight))
		}
		if !c.HasRules() {
			add("expect", "caso sem regra: preencha expect (must_match/must_not_match/json_schema) ou judge")
		}
		if len(c.Expect.JSONSchema) > 0 {
			if err := compileSchema(c.Expect.JSONSchema); err != nil {
				add("expect.json_schema", err.Error())
			}
		}
		if c.Judge != nil {
			if strings.TrimSpace(c.Judge.Rubric) == "" {
				add("judge.rubric", "rubric vazia")
			}
			if c.Judge.MaxScore <= 0 {
				add("judge.max_score", fmt.Sprintf("max_score deve ser > 0 (recebido %v)", c.Judge.MaxScore))
			}
			if c.Judge.Threshold <= 0 || c.Judge.Threshold > 1 {
				add("judge.threshold", fmt.Sprintf("threshold deve estar em (0,1] como fracao de max_score (recebido %v)", c.Judge.Threshold))
			}
		}
	}
	return issues
}

// compileSchema valida o fragmento JSON Schema usado pela regra de expect.
func compileSchema(raw json.RawMessage) error {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("json_schema invalido: %w", err)
	}
	compiler := jsonschemav6.NewCompiler()
	if err := compiler.AddResource("case-json-schema", doc); err != nil {
		return fmt.Errorf("json_schema nao registrado: %w", err)
	}
	if _, err := compiler.Compile("case-json-schema"); err != nil {
		return fmt.Errorf("json_schema nao compila: %w", err)
	}
	return nil
}

// RequiredFields devolve os nomes em `required` de nivel 1 do json_schema do
// caso. Sem json_schema (ou sem required) devolve nil.
func (c Case) RequiredFields() []string {
	if len(c.Expect.JSONSchema) == 0 {
		return nil
	}
	var doc struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(c.Expect.JSONSchema, &doc); err != nil {
		return nil
	}
	return doc.Required
}
