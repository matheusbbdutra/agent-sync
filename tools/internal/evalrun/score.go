// Package evalrun pontua suites de avaliacao de forma deterministica, compara
// com um baseline e registra cada execucao em JSONL append-only.
// Os tipos de registro (Totals, SliceStats, RunRecord) vivem em record.go.
//
// Divisao de responsabilidade entre os arquivos do pacote:
//
//	score.go   — pontuacao de um caso (must_match, must_not_match, json_schema)
//	             + invocacao opcional de um juiz externo (judge-cmd).
//	compare.go — comparacao com baseline por slice e deteccao de regressao.
//	record.go  — hashes sha256, run_id e o registro append-only.
package evalrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/matheusdutra/token-tools/internal/evalsuite"
)

// Status possiveis de um caso. Um caso sem regra aplicavel fica "skipped":
// nunca vira "pass", porque ausencia de regra nao e evidencia de acerto.
const (
	StatusPass    = "pass"
	StatusFail    = "fail"
	StatusSkipped = "skipped"
)

// RuleResult e o resultado de uma regra deterministica.
type RuleResult struct {
	Rule    string `json:"rule"` // must_match | must_not_match | json_schema
	Pattern string `json:"pattern,omitempty"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
}

// JudgeVerdict guarda o veredito do juiz externo, incluindo a normalizacao
// explicita (score / max_score) usada na comparacao com o threshold.
type JudgeVerdict struct {
	Score      float64 `json:"score"`
	MaxScore   float64 `json:"max_score"`
	Normalized float64 `json:"normalized"`
	Threshold  float64 `json:"threshold"`
	Pass       bool    `json:"pass"`
	Reason     string  `json:"reason,omitempty"`
}

// CaseResult agrega o que aconteceu em um caso.
type CaseResult struct {
	CaseID       string        `json:"case_id"`
	Slice        string        `json:"slice"`
	Weight       float64       `json:"weight"`
	Status       string        `json:"status"`
	Rules        []RuleResult  `json:"rules,omitempty"`
	Judge        *JudgeVerdict `json:"judge,omitempty"`
	JudgeSkipped bool          `json:"judge_skipped,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}

// Judge atribui nota a um caso. Isolar a invocacao externa atras desta
// interface mantem o scoring deterministico testavel sem spawnar processo.
type Judge interface {
	Score(c evalsuite.Case, output string) (score float64, reason string, err error)
}

// ExecJudge roda um processo externo por caso: escreve {"case":...,"output":...}
// em stdin e espera {"score":N,"reason":"..."} em stdout. Sem SDK de provider e
// sem HTTP — o juiz e responsabilidade de quem passa --judge-cmd.
type ExecJudge struct {
	Command string
	Timeout time.Duration
}

// Score implementa Judge. Erro aqui e erro de configuracao/infra do juiz, nao
// veredito: o caller deve abortar a execucao (exit 2) em vez de marcar pass.
func (j ExecJudge) Score(c evalsuite.Case, output string) (float64, string, error) {
	payload, err := json.Marshal(map[string]any{"case": c, "output": output})
	if err != nil {
		return 0, "", fmt.Errorf("empacotar payload do juiz: %w", err)
	}
	timeout := j.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", j.Command)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, "", fmt.Errorf("judge-cmd falhou: %w (stderr: %s)", err, truncate(stderr.String(), 400))
	}

	var resp struct {
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &resp); err != nil {
		return 0, "", fmt.Errorf("judge-cmd devolveu JSON invalido: %w (stdout: %s)", err, truncate(stdout.String(), 400))
	}
	return resp.Score, resp.Reason, nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ScoreCase aplica as regras do caso ao output. Regra deterministica que falha
// encerra o caso como fail. Sem regra aplicavel (nenhuma regra deterministica e
// juiz ausente) o caso fica skipped. Erro so acontece se o juiz falhar em
// executar/responder.
func ScoreCase(c evalsuite.Case, output string, judge Judge) (CaseResult, error) {
	res := CaseResult{CaseID: c.ID, Slice: c.Slice, Weight: c.Weight}

	for _, pattern := range c.Expect.MustMatch {
		ok := containsFold(output, pattern)
		res.Rules = append(res.Rules, RuleResult{Rule: "must_match", Pattern: pattern, OK: ok})
	}
	for _, pattern := range c.Expect.MustNotMatch {
		ok := !containsFold(output, pattern)
		res.Rules = append(res.Rules, RuleResult{Rule: "must_not_match", Pattern: pattern, OK: ok})
	}
	if required := c.RequiredFields(); len(required) > 0 {
		res.Rules = append(res.Rules, jsonSchemaRule(output, required))
	}

	for _, rule := range res.Rules {
		if !rule.OK {
			res.Status = StatusFail
			res.Reason = "regra deterministica falhou: " + ruleLabel(rule)
			return res, nil
		}
	}

	hasJudge := c.Judge != nil
	if !hasJudge && len(res.Rules) == 0 {
		res.Status = StatusSkipped
		res.Reason = "caso sem regra aplicavel"
		return res, nil
	}
	if !hasJudge {
		res.Status = StatusPass
		return res, nil
	}
	if judge == nil {
		// Regra de juiz existe mas nao foi executada: nao inventa veredito.
		if len(res.Rules) == 0 {
			res.Status = StatusSkipped
			res.Reason = "caso so tem regra de judge e --judge-cmd nao foi informado"
			return res, nil
		}
		res.JudgeSkipped = true
		res.Status = StatusPass
		res.Reason = "judge nao executado (--judge-cmd ausente); veredito so deterministico"
		return res, nil
	}

	score, reason, err := judge.Score(c, output)
	if err != nil {
		return res, err
	}
	normalized := score / c.Judge.MaxScore
	verdict := &JudgeVerdict{
		Score:      score,
		MaxScore:   c.Judge.MaxScore,
		Normalized: normalized,
		Threshold:  c.Judge.Threshold,
		Pass:       normalized >= c.Judge.Threshold,
		Reason:     reason,
	}
	res.Judge = verdict
	if !verdict.Pass {
		res.Status = StatusFail
		res.Reason = fmt.Sprintf("judge score %.3f/%.3f = %.3f abaixo do threshold %.3f",
			score, c.Judge.MaxScore, normalized, c.Judge.Threshold)
		return res, nil
	}
	res.Status = StatusPass
	return res, nil
}

func ruleLabel(r RuleResult) string {
	if r.Pattern == "" {
		return r.Rule
	}
	return r.Rule + "(" + r.Pattern + ")"
}

// containsFold faz busca de substring sem diferenciar maiusculas/minusculas.
func containsFold(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// jsonSchemaRule checa apenas o nivel 1 do schema: o output precisa ser JSON
// valido e conter todas as chaves de `required`. Palavras-chave mais profundas
// do schema nao sao avaliadas aqui (ver reference/eval-harness.md).
func jsonSchemaRule(output string, required []string) RuleResult {
	rule := RuleResult{Rule: "json_schema", Pattern: strings.Join(required, ",")}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &obj); err != nil {
		rule.Detail = "output nao e JSON objeto: " + err.Error()
		return rule
	}
	var missing []string
	for _, key := range required {
		if _, ok := obj[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		rule.Detail = "campos obrigatorios ausentes: " + strings.Join(missing, ",")
		return rule
	}
	rule.OK = true
	return rule
}

// Aggregate soma os resultados em Totals + uma entrada por slice, ordenada por
// nome. PassRate de um slice = soma de weight dos pass / soma de weight dos
// scored; casos skipped ficam fora do denominador.
func Aggregate(results []CaseResult) (Totals, []SliceStats) {
	totals := Totals{Cases: len(results)}
	bySlice := map[string]*SliceStats{}
	for _, r := range results {
		slice := r.Slice
		if slice == "" {
			slice = "(sem slice)"
		}
		st, ok := bySlice[slice]
		if !ok {
			st = &SliceStats{Slice: slice}
			bySlice[slice] = st
		}
		st.Cases++
		switch r.Status {
		case StatusPass:
			totals.Pass++
			st.Pass++
			st.WeightPass += r.Weight
		case StatusFail:
			totals.Fail++
			st.Fail++
		default:
			totals.Skipped++
			st.Skipped++
			continue
		}
		totals.Scored++
		st.Scored++
		st.WeightScored += r.Weight
	}

	slices := make([]SliceStats, 0, len(bySlice))
	for _, st := range bySlice {
		if st.WeightScored > 0 {
			st.PassRate = st.WeightPass / st.WeightScored
		}
		slices = append(slices, *st)
	}
	sort.Slice(slices, func(i, j int) bool { return slices[i].Slice < slices[j].Slice })
	return totals, slices
}
