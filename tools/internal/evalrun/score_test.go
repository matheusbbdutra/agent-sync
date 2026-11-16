package evalrun

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/matheusdutra/token-tools/internal/evalsuite"
)

// fakeJudge devolve score fixo e registra o output recebido.
type fakeJudge struct {
	score  float64
	reason string
	err    error
	calls  int
}

func (f *fakeJudge) Score(c evalsuite.Case, output string) (float64, string, error) {
	f.calls++
	return f.score, f.reason, f.err
}

func judgeCase(threshold float64) evalsuite.Case {
	return evalsuite.Case{
		ID: "j", Slice: "s", Weight: 1,
		Judge: &evalsuite.Judge{Rubric: "diga se a resposta esta correta", MaxScore: 5, Threshold: threshold},
	}
}

func TestScoreCaseMustMatchIsCaseInsensitive(t *testing.T) {
	c := evalsuite.Case{ID: "a", Slice: "s", Weight: 1, Expect: evalsuite.Expect{MustMatch: []string{"Repo-Auditor"}}}
	res, err := ScoreCase(c, "chame a skill REPO-AUDITOR agora", nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusPass {
		t.Fatalf("status = %s, quer pass (%s)", res.Status, res.Reason)
	}
}

func TestScoreCaseMustMatchFailsWhenAbsent(t *testing.T) {
	c := evalsuite.Case{ID: "a", Slice: "s", Weight: 1, Expect: evalsuite.Expect{MustMatch: []string{"debugging-strategies"}}}
	res, err := ScoreCase(c, "chame a skill repo-auditor", nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusFail {
		t.Fatalf("status = %s, quer fail", res.Status)
	}
	if len(res.Rules) != 1 || res.Rules[0].OK {
		t.Fatalf("rules = %+v, quer 1 regra falhando", res.Rules)
	}
}

func TestScoreCaseMustNotMatchFailsWhenPresent(t *testing.T) {
	c := evalsuite.Case{ID: "a", Slice: "s", Weight: 1, Expect: evalsuite.Expect{
		MustMatch:    []string{"repo-auditor"},
		MustNotMatch: []string{"security-auditor"},
	}}
	res, err := ScoreCase(c, "use repo-auditor e security-auditor", nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusFail {
		t.Fatalf("status = %s, quer fail", res.Status)
	}
}

func TestScoreCaseJSONSchemaLevel1(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["skill"],"properties":{"skill":{"type":"string"}}}`)
	c := evalsuite.Case{ID: "a", Slice: "s", Weight: 1, Expect: evalsuite.Expect{JSONSchema: schema}}

	pass, err := ScoreCase(c, `{"skill":"repo-auditor","motivo":"x"}`, nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if pass.Status != StatusPass {
		t.Fatalf("status = %s, quer pass (%s)", pass.Status, pass.Reason)
	}

	missing, _ := ScoreCase(c, `{"motivo":"x"}`, nil)
	if missing.Status != StatusFail {
		t.Fatalf("campo obrigatorio ausente: status = %s, quer fail", missing.Status)
	}

	broken, _ := ScoreCase(c, `nao e json`, nil)
	if broken.Status != StatusFail {
		t.Fatalf("output nao-JSON: status = %s, quer fail", broken.Status)
	}
}

func TestScoreCaseSkippedWhenOnlyJudgeAndNoJudgeCmd(t *testing.T) {
	res, err := ScoreCase(judgeCase(0.6), "qualquer coisa", nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("status = %s, quer skipped: caso sem regra aplicavel nunca vira pass", res.Status)
	}
}

func TestScoreCaseDeterministicWinsWhenJudgeMissing(t *testing.T) {
	c := judgeCase(0.6)
	c.Expect.MustMatch = []string{"ok"}
	res, err := ScoreCase(c, "esta ok", nil)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusPass || !res.JudgeSkipped {
		t.Fatalf("status = %s judge_skipped = %v, quer pass com judge_skipped", res.Status, res.JudgeSkipped)
	}
}

func TestScoreCaseJudgeThreshold(t *testing.T) {
	above := &fakeJudge{score: 4}
	res, err := ScoreCase(judgeCase(0.6), "saida", above)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusPass || res.Judge == nil {
		t.Fatalf("status = %s, quer pass com veredito do judge", res.Status)
	}
	if res.Judge.Normalized != 0.8 {
		t.Errorf("normalized = %v, quer 0.8 (4/5)", res.Judge.Normalized)
	}
	if above.calls != 1 {
		t.Errorf("judge chamado %d vezes, quer 1", above.calls)
	}

	below := &fakeJudge{score: 2.5}
	res, err = ScoreCase(judgeCase(0.6), "saida", below)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusFail {
		t.Fatalf("status = %s, quer fail (2.5/5 = 0.5 < 0.6)", res.Status)
	}
}

func TestScoreCaseJudgeNotCalledWhenDeterministicFails(t *testing.T) {
	j := &fakeJudge{score: 5}
	c := judgeCase(0.6)
	c.Expect.MustMatch = []string{"obrigatorio"}
	res, err := ScoreCase(c, "saida sem o termo", j)
	if err != nil {
		t.Fatalf("ScoreCase: %v", err)
	}
	if res.Status != StatusFail {
		t.Fatalf("status = %s, quer fail", res.Status)
	}
	if j.calls != 0 {
		t.Errorf("judge chamado %d vezes, quer 0 (falha deterministica encerra o caso)", j.calls)
	}
}

func TestScoreCaseJudgeErrorIsError(t *testing.T) {
	_, err := ScoreCase(judgeCase(0.6), "saida", &fakeJudge{err: errors.New("processo morreu")})
	if err == nil {
		t.Fatal("falha do judge deveria propagar erro, nao virar veredito")
	}
}

func TestExecJudgeParsesJSON(t *testing.T) {
	j := ExecJudge{Command: `cat >/dev/null; printf '{"score":3.5,"reason":"ok"}'`}
	score, reason, err := j.Score(judgeCase(0.5), "saida")
	if err != nil {
		t.Fatalf("ExecJudge.Score: %v", err)
	}
	if score != 3.5 || reason != "ok" {
		t.Fatalf("score = %v reason = %q, quer 3.5/ok", score, reason)
	}
}

func TestExecJudgeRejectsInvalidJSON(t *testing.T) {
	j := ExecJudge{Command: `cat >/dev/null; echo nao-json`}
	if _, _, err := j.Score(judgeCase(0.5), "saida"); err == nil {
		t.Fatal("stdout invalido deveria gerar erro")
	}
}

func TestExecJudgeReceivesCaseAndOutput(t *testing.T) {
	// O juiz imprime o proprio payload de volta como score textual: prova que
	// {"case":...,"output":...} chega em stdin.
	j := ExecJudge{Command: `payload=$(cat); case "$payload" in *'"output":"marca-unica"'*) printf '{"score":5}';; *) printf '{"score":0}';; esac`}
	score, _, err := j.Score(judgeCase(0.5), "marca-unica")
	if err != nil {
		t.Fatalf("ExecJudge.Score: %v", err)
	}
	if score != 5 {
		t.Fatalf("score = %v, quer 5 (payload nao chegou em stdin)", score)
	}
}

func TestAggregateWeightsAndSlices(t *testing.T) {
	results := []CaseResult{
		{CaseID: "a", Slice: "s1", Weight: 1, Status: StatusPass},
		{CaseID: "b", Slice: "s1", Weight: 3, Status: StatusFail},
		{CaseID: "c", Slice: "s2", Weight: 2, Status: StatusSkipped},
		{CaseID: "d", Slice: "s2", Weight: 1, Status: StatusPass},
	}
	totals, slices := Aggregate(results)

	if totals.Cases != 4 || totals.Scored != 3 || totals.Skipped != 1 || totals.Pass != 2 || totals.Fail != 1 {
		t.Fatalf("totals = %+v, quer cases4 scored3 skipped1 pass2 fail1", totals)
	}
	if len(slices) != 2 {
		t.Fatalf("slices = %d, quer 2", len(slices))
	}
	if slices[0].Slice != "s1" || slices[1].Slice != "s2" {
		t.Fatalf("ordem dos slices = %v, quer s1,s2", slices)
	}
	// s1: pass weight 1 / scored weight 4 = 0.25 (o fail de weight 3 pesa).
	if slices[0].PassRate != 0.25 {
		t.Errorf("pass_rate s1 = %v, quer 0.25", slices[0].PassRate)
	}
	if slices[0].WeightScored != 4 {
		t.Errorf("weight_scored s1 = %v, quer 4", slices[0].WeightScored)
	}
	// s2: skipped fica fora do denominador => 1/1 = 1.0.
	if slices[1].PassRate != 1 {
		t.Errorf("pass_rate s2 = %v, quer 1 (skipped fora do denominador)", slices[1].PassRate)
	}
}
