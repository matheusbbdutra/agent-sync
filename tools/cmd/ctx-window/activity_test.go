package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitTurn(t *testing.T) {
	cases := []struct {
		input    string
		wantTool string
		wantRest string
	}{
		{"Read: foo.go", "Read", "foo.go"},
		{"Bash: kubectl get pods", "Bash", "kubectl get pods"},
		{"Read:", "Read", ""},
		{"no-colon-here", "no-colon-here", ""},
		{"  Edit  :  content  ", "Edit", "content"},
		{"", "", ""},
	}
	for _, c := range cases {
		tool, rest := splitTurn(c.input)
		if tool != c.wantTool {
			t.Errorf("splitTurn(%q) tool=%q, want %q", c.input, tool, c.wantTool)
		}
		if rest != c.wantRest {
			t.Errorf("splitTurn(%q) rest=%q, want %q", c.input, rest, c.wantRest)
		}
	}
}

func TestSortActivityCounts(t *testing.T) {
	cats := []ActivityCount{
		{Mode: "code", Count: 3},
		{Mode: "debug", Count: 3},
		{Mode: "infra", Count: 5},
		{Mode: "review", Count: 3},
	}
	sortActivityCounts(cats)
	if cats[0].Mode != "infra" {
		t.Errorf("top should be infra, got %s", cats[0].Mode)
	}
	// desempate lexicográfico entre code/debug/review (todos com count=3)
	rest := []string{cats[1].Mode, cats[2].Mode, cats[3].Mode}
	want := []string{"code", "debug", "review"}
	for i := range rest {
		if rest[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i+1, rest[i], want[i])
		}
	}
}

func TestClassifyActivityEmpty(t *testing.T) {
	got := ClassifyActivity(nil)
	if got.Mode != "general" || got.Score != 0 || got.Window != 0 {
		t.Errorf("empty turns should give general/0/0, got %+v", got)
	}
}

func TestClassifyActivityCode(t *testing.T) {
	turns := []Turn{
		{Content: "Edit: foo.go"},
		{Content: "Write: bar.go"},
		{Content: "NotebookEdit: cells"},
		{Content: "Read: foo.go"},
	}
	got := ClassifyActivity(turns)
	if got.Mode != "code" {
		t.Errorf("expected code, got %s", got.Mode)
	}
	if got.Score <= 0 {
		t.Errorf("expected positive score, got %d", got.Score)
	}
}

func TestClassifyActivityDebug(t *testing.T) {
	turns := []Turn{
		{Content: "Bash: go test ./..."},
		{Content: "Bash: error in test"},
		{Content: "Grep: panic in stack trace"},
	}
	got := ClassifyActivity(turns)
	if got.Mode != "debug" {
		t.Errorf("expected debug, got %s (cats=%+v)", got.Mode, got.Categories)
	}
}

func TestClassifyActivityInfra(t *testing.T) {
	turns := []Turn{
		{Content: "Bash: kubectl get pods"},
		{Content: "Bash: docker compose up"},
		{Content: "Bash: terraform apply"},
	}
	got := ClassifyActivity(turns)
	if got.Mode != "infra" {
		t.Errorf("expected infra, got %s (cats=%+v)", got.Mode, got.Categories)
	}
}

func TestClassifyActivityReview(t *testing.T) {
	turns := []Turn{
		{Content: "Read: main.go"},
		{Content: "Read: store.go"},
		{Content: "Read: hook.go"},
		{Content: "Glob: *.go"},
	}
	got := ClassifyActivity(turns)
	if got.Mode != "review" {
		t.Errorf("expected review, got %s (cats=%+v)", got.Mode, got.Categories)
	}
}

func TestClassifyActivityRespectsWindow(t *testing.T) {
	// 15 turns: 5 code (antigas) + 10 debug (recentes) → janela 10 pega só debug
	turns := make([]Turn, 15)
	for i := 0; i < 5; i++ {
		turns[i] = Turn{Content: "Edit: file.go"}
	}
	for i := 5; i < 15; i++ {
		turns[i] = Turn{Content: "Bash: error in test panic"}
	}
	got := ClassifyActivity(turns)
	if got.Mode != "debug" {
		t.Errorf("window should exclude the first 5 code turns, got %s", got.Mode)
	}
	if got.Window != activityWindowSize {
		t.Errorf("window=%d, want %d", got.Window, activityWindowSize)
	}
}

func TestClassifyActivityDistributionSum(t *testing.T) {
	// soma das counts deve ser >= total de turns (cada turno dá pontos)
	turns := []Turn{
		{Content: "Edit: foo.go"},
		{Content: "Bash: error test"},
		{Content: "Read: bar.go"},
	}
	got := ClassifyActivity(turns)
	total := 0
	for _, c := range got.Categories {
		total += c.Count
	}
	if total == 0 {
		t.Error("distribution should be non-empty for non-empty turns")
	}
}

func TestRunActivityMissingSession(t *testing.T) {
	withTempCache(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"activity"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error when session is missing")
	}
}

func TestRunActivityPlain(t *testing.T) {
	withTempCache(t)
	s, _ := Load("act-sess")
	s.AddTurn(Turn{Content: "Edit: foo.go"})
	s.AddTurn(Turn{Content: "Edit: bar.go"})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"activity", "act-sess"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	for _, want := range []string{"activity for last", "mode:", "score:", "distribution:", "code"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestRunActivityJSON(t *testing.T) {
	withTempCache(t)
	s, _ := Load("act-json")
	s.AddTurn(Turn{Content: "Bash: kubectl apply"})
	var stdout, stderr bytes.Buffer
	if err := run([]string{"activity", "--json", "act-json"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var a Activity
	if err := json.Unmarshal(stdout.Bytes(), &a); err != nil {
		t.Fatalf("invalid JSON: %v\nout=%s", err, stdout.String())
	}
	if a.Mode != "infra" {
		t.Errorf("expected infra mode, got %s", a.Mode)
	}
	if len(a.Categories) == 0 {
		t.Error("expected non-empty categories")
	}
}
