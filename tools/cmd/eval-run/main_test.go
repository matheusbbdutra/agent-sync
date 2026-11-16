package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runMain(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// lastRunID le o run_id da ultima linha do registro (o registro e append-only).
func lastRunID(t *testing.T, recordPath string) string {
	t.Helper()
	raw, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("ler registro %s: %v", recordPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var rec struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &rec); err != nil {
		t.Fatalf("registro invalido: %v", err)
	}
	return rec.RunID
}

const testSuite = `{"id":"rotea-01","slice":"pt","weight":1,"input":{"kind":"prompt","text":"audite o repo"},"expect":{"must_match":["repo-auditor"]}}
{"id":"rotea-02","slice":"pt","weight":1,"input":{"kind":"prompt","text":"revise o mr"},"expect":{"must_match":["cli-mr-review"]}}
{"id":"aberto-03","slice":"aberto","weight":1,"input":{"kind":"prompt","text":"resuma"},"judge":{"rubric":"r","max_score":5,"threshold":0.6}}
`

func TestUsageErrors(t *testing.T) {
	if code, _, stderr := run(t); code != exitUsage || !strings.Contains(stderr, "Usage") {
		t.Fatalf("sem args: code=%d stderr=%q, quer 2 com usage", code, stderr)
	}
	if code, _, _ := run(t, "subcomando-inexistente"); code != exitUsage {
		t.Fatalf("subcomando desconhecido: code=%d, quer 2", code)
	}
	if code, _, _ := run(t, "run"); code != exitUsage {
		t.Fatalf("run sem flags: code=%d, quer 2", code)
	}
	if code, _, _ := run(t, "suite", "check"); code != exitUsage {
		t.Fatalf("suite check sem path: code=%d, quer 2", code)
	}
}

func TestSuiteCheckValid(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	writeFile(t, suite, testSuite)

	code, stdout, stderr := run(t, "suite", "check", suite)
	if code != exitOK {
		t.Fatalf("code=%d, quer 0 (stderr=%s)", code, stderr)
	}
	if !strings.Contains(stdout, "casos 3") || !strings.Contains(stdout, "slices 2") {
		t.Fatalf("stdout=%q, quer casos 3 e slices 2", stdout)
	}
}

func TestSuiteCheckJSON(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	writeFile(t, suite, testSuite)

	code, stdout, _ := run(t, "suite", "check", "--json", suite)
	if code != exitOK {
		t.Fatalf("code=%d, quer 0", code)
	}
	var got suiteCheckReport
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("saida nao e JSON: %v (%q)", err, stdout)
	}
	if !got.Valid || got.Cases != 3 {
		t.Fatalf("report=%+v, quer valid com 3 casos", got)
	}
}

func TestSuiteCheckInvalidSuite(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	writeFile(t, suite, `{"id":"","slice":"","input":{"kind":"k","text":"t"}}`+"\n")

	code, _, stderr := run(t, "suite", "check", suite)
	if code != exitUsage {
		t.Fatalf("code=%d, quer 2 para suite invalida", code)
	}
	if !strings.Contains(stderr, "suite invalida") {
		t.Fatalf("stderr=%q, quer citar 'suite invalida'", stderr)
	}
}

func TestRunPassesAndRecords(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	record := filepath.Join(dir, ".agent-sync", "eval-runs.jsonl")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	code, stdout, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record)
	if code != exitOK {
		t.Fatalf("code=%d, quer 0 (stdout=%s stderr=%s)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "pass 2") || !strings.Contains(stdout, "skipped 1") {
		t.Fatalf("stdout=%q, quer pass 2 e skipped 1 (caso so com judge fica skipped)", stdout)
	}
	if !strings.Contains(stdout, "registro "+record) {
		t.Fatalf("stdout=%q, quer registrar o caminho do registro", stdout)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("registro nao gravado: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(bytes.SplitN(raw, []byte("\n"), 2)[0], &rec); err != nil {
		t.Fatalf("registro nao e JSON: %v", err)
	}
	for _, key := range []string{"schema_version", "run_id", "ts", "suite", "suite_hash", "target", "target_hash", "model", "totals", "by_slice"} {
		if _, ok := rec[key]; !ok {
			t.Errorf("registro sem campo %q: %v", key, rec)
		}
	}
	if !strings.HasPrefix(rec["suite_hash"].(string), "sha256:") {
		t.Errorf("suite_hash = %v, quer prefixo sha256:", rec["suite_hash"])
	}
}

func TestRunFailsWhenCaseFails(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	record := filepath.Join(dir, "runs.jsonl")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor")

	code, stdout, _ := run(t, "run", "--suite", suite, "--target", target, "--record", record)
	if code != exitFailures {
		t.Fatalf("code=%d, quer 1 com caso em fail", code)
	}
	if !strings.Contains(stdout, "fail  rotea-02") {
		t.Fatalf("stdout=%q, quer listar o caso que falhou", stdout)
	}
}

func TestRunSliceFilter(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor")

	code, stdout, _ := run(t, "run", "--suite", suite, "--target", target, "--slice", "pt", "--no-record", "--record", filepath.Join(dir, "runs.jsonl"))
	if code != exitFailures {
		t.Fatalf("code=%d, quer 1 (1 de 2 casos do slice pt falha)", code)
	}
	if !strings.Contains(stdout, "casos 2") || strings.Contains(stdout, "skipped 1") {
		t.Fatalf("stdout=%q, quer apenas os 2 casos do slice pt", stdout)
	}
	if !strings.Contains(stdout, "--no-record") {
		t.Fatalf("stdout=%q, quer indicar que o registro nao foi gravado", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "runs.jsonl")); !os.IsNotExist(err) {
		t.Fatal("--no-record deveria impedir a gravacao")
	}

	if code, _, _ := run(t, "run", "--suite", suite, "--target", target, "--slice", "inexistente", "--no-record", "--record", filepath.Join(dir, "runs.jsonl")); code != exitUsage {
		t.Fatalf("slice inexistente: code=%d, quer 2", code)
	}
}

func TestRunIncomparableBaseline(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	record := filepath.Join(dir, "runs.jsonl")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	if code, _, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record); code != exitOK {
		t.Fatalf("primeira run: code=%d stderr=%s", code, stderr)
	}
	// Target muda => hash muda => incomparavel.
	writeFile(t, target, "chame repo-auditor e cli-mr-review agora")
	firstRunID := lastRunID(t, record)
	code, stdout, _ := run(t, "run", "--suite", suite, "--target", target, "--record", record, "--baseline", firstRunID)
	if code != exitIncomparable {
		t.Fatalf("code=%d, quer 3 (target_hash diverge)", code)
	}
	if !strings.Contains(stdout, "incomparavel") {
		t.Fatalf("stdout=%q, quer declarar incomparavel", stdout)
	}
}

func TestRunComparableBaseline(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "alvo.md")
	record := filepath.Join(dir, "runs.jsonl")
	writeFile(t, suite, testSuite)
	// Mesmo conteudo em outro caminho: target_hash igual => comparavel.
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	if code, _, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record); code != exitOK {
		t.Fatalf("baseline: code=%d stderr=%s", code, stderr)
	}

	code, stdout, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record, "--baseline", lastRunID(t, record))
	if code != exitOK {
		t.Fatalf("code=%d, quer 0 com baseline comparavel (stdout=%s stderr=%s)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "delta_pass_rate +0.000") {
		t.Fatalf("stdout=%q, quer delta 0 contra o mesmo target", stdout)
	}

	if code, _, _ := run(t, "run", "--suite", suite, "--target", target, "--record", record, "--baseline", "eval-inexistente"); code != exitUsage {
		t.Fatalf("baseline inexistente: code=%d, quer 2", code)
	}
}

func TestReportAndList(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	record := filepath.Join(dir, "runs.jsonl")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	if code, _, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record); code != exitOK {
		t.Fatalf("run 1: code=%d stderr=%s", code, stderr)
	}
	if code, _, stderr := run(t, "run", "--suite", suite, "--target", target, "--record", record); code != exitOK {
		t.Fatalf("run 2: code=%d stderr=%s", code, stderr)
	}

	code, stdout, stderr := run(t, "report", "--record", record)
	if code != exitOK {
		t.Fatalf("report: code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "casos 3") {
		t.Fatalf("report=%q, quer resumo de casos", stdout)
	}

	code, stdout, _ = run(t, "report", "--record", record, "--json")
	if code != exitOK {
		t.Fatalf("report --json: code=%d", code)
	}
	var one map[string]any
	if err := json.Unmarshal([]byte(stdout), &one); err != nil {
		t.Fatalf("report --json invalido: %v (%q)", err, stdout)
	}
	if one["run_id"] == "" {
		t.Fatalf("report sem run_id: %v", one)
	}

	code, stdout, _ = run(t, "list", "--record", record, "--json")
	if code != exitOK {
		t.Fatalf("list --json: code=%d", code)
	}
	var many []map[string]any
	if err := json.Unmarshal([]byte(stdout), &many); err != nil {
		t.Fatalf("list --json invalido: %v (%q)", err, stdout)
	}
	if len(many) != 2 {
		t.Fatalf("list = %d registros, quer 2", len(many))
	}

	code, stdout, _ = run(t, "list", "--record", record)
	if code != exitOK || !strings.Contains(stdout, "run_id") {
		t.Fatalf("list texto: code=%d stdout=%q", code, stdout)
	}

	if code, _, _ := run(t, "report", "--record", record, "--run", "eval-inexistente"); code != exitUsage {
		t.Fatalf("report com run inexistente: code=%d, quer 2", code)
	}
	if code, _, _ := run(t, "report", "--record", filepath.Join(dir, "vazio.jsonl")); code != exitUsage {
		t.Fatalf("report sem registros: code=%d, quer 2", code)
	}
}

func TestRunJudgeCmd(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	// Juiz aprova sempre com score maximo: o caso aberto-03 vira pass.
	code, stdout, stderr := run(t, "run", "--suite", suite, "--target", target,
		"--record", filepath.Join(dir, "runs.jsonl"), "--slice", "aberto",
		"--judge-cmd", `cat >/dev/null; printf '{"score":5,"reason":"ok"}'`)
	if code != exitOK {
		t.Fatalf("code=%d, stdout=%s stderr=%s, quer 0", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "pass 1") {
		t.Fatalf("stdout=%q, quer pass 1 pelo judge", stdout)
	}
}

func TestRunJudgeCmdFailureIsUsageError(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.jsonl")
	target := filepath.Join(dir, "target.md")
	writeFile(t, suite, testSuite)
	writeFile(t, target, "chame repo-auditor e cli-mr-review")

	code, _, stderr := run(t, "run", "--suite", suite, "--target", target,
		"--slice", "aberto", "--no-record", "--record", filepath.Join(dir, "runs.jsonl"),
		"--judge-cmd", `cat >/dev/null; exit 7`)
	if code != exitUsage {
		t.Fatalf("code=%d, quer 2 (juiz quebrado nao pode virar veredito)", code)
	}
	if !strings.Contains(stderr, "judge-cmd") {
		t.Fatalf("stderr=%q, quer citar judge-cmd", stderr)
	}
}
