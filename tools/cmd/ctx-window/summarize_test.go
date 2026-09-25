package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matheusdutra/token-tools/internal/agentmemory"
)

func TestSummarizerCommandDispatchesPerCLI(t *testing.T) {
	cases := []struct {
		cli       string
		wantBin   string
		wantParts []string
	}{
		{"claude", "claude", []string{"-p", "--model", "opus", "hello"}},
		{"codex", "codex", []string{"exec", "--model", "opus", "hello"}},
		{"opencode", "opencode", []string{"run", "--pure", "-m", "opus", "hello"}},
		{"cursor", "cursor-agent", []string{"-p", "--model", "opus", "hello"}},
		{"antigravity", "agy", []string{"-p", "--model", "opus", "hello"}},
	}
	for _, tc := range cases {
		t.Run(tc.cli, func(t *testing.T) {
			cmd, err := summarizerCommand(context.Background(), tc.cli, "opus", "hello")
			if err != nil {
				t.Fatalf("summarizerCommand(%q): %v", tc.cli, err)
			}
			if got := cmd.Args[0]; !strings.HasSuffix(got, tc.wantBin) {
				t.Errorf("bin = %q, want suffix %q", got, tc.wantBin)
			}
			got := cmd.Args[1:]
			if len(got) != len(tc.wantParts) {
				t.Fatalf("args = %v, want %v", got, tc.wantParts)
			}
			for i, part := range tc.wantParts {
				if got[i] != part {
					t.Errorf("args[%d] = %q, want %q (full: %v)", i, got[i], part, got)
				}
			}
		})
	}
}

func TestSummarizerCommandNoModelOmitsFlag(t *testing.T) {
	cmd, err := summarizerCommand(context.Background(), "claude", "", "hello")
	if err != nil {
		t.Fatalf("summarizerCommand: %v", err)
	}
	want := []string{"-p", "hello"}
	got := cmd.Args[1:]
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

func TestSummarizerCommandUnknownCLI(t *testing.T) {
	if _, err := summarizerCommand(context.Background(), "gemini-cli", "", "hello"); err == nil {
		t.Fatal("expected error for unknown CLI, got nil")
	}
}

func TestSummarizeHeuristicSkipsLLM(t *testing.T) {
	withTempCache(t)
	t.Setenv("AGENT_SYNC_SUMMARIZER", "heuristic")
	s, err := Load("heur-sess")
	if err != nil {
		t.Fatal(err)
	}
	s.ProjectPath = t.TempDir() // evita gravar .agent-sync/ no repo
	if err := s.AddTurn(Turn{Role: "tool", Content: "decidimos usar FastAPI para a API"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	// Sem --cli e sem cli_name: o caminho heurístico não precisa de CLI
	// (o LLM retornaria erro "no CLI known").
	if err := runSummarize([]string{"heur-sess"}, &stdout, &stderr); err != nil {
		t.Fatalf("runSummarize heuristic: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "summarizer=heuristic") {
		t.Errorf("stdout should report heuristic, got: %s", stdout.String())
	}
	s2, err := Load("heur-sess")
	if err != nil {
		t.Fatal(err)
	}
	if s2.Version != 1 {
		t.Errorf("version should advance to 1, got %d", s2.Version)
	}
	dir, err := SessionDir("heur-sess")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(summaryPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "FastAPI") {
		t.Errorf("summary.md should carry the decision marker, got: %s", body)
	}
}

func TestChooseHeuristic(t *testing.T) {
	small := &Session{Turns: make([]Turn, 3)}
	big := &Session{Turns: make([]Turn, autoHeuristicMaxTurns+1)}
	for _, tc := range []struct {
		mode string
		s    *Session
		want bool
	}{
		{"heuristic", big, true},
		{"auto", small, true},
		{"auto", big, false},
		{"agent", small, false},
		{"ollama:qwen2.5:3b", small, false},
		{"", small, false},
	} {
		if got := chooseHeuristic(tc.mode, tc.s); got != tc.want {
			t.Errorf("chooseHeuristic(%q, %d turns)=%v, want %v", tc.mode, len(tc.s.Turns), got, tc.want)
		}
	}
}

func TestSummarizerFromConfigReadsConfigJSON(t *testing.T) {
	// Campo "summarizer" do config.json é lido quando a env está vazia.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AGENT_SYNC_SUMMARIZER", "")
	if err := os.MkdirAll(filepath.Join(dir, "agent-sync"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "agent-sync", "config.json")
	if err := os.WriteFile(cfg, []byte(`{"summarizer":"auto"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SummarizerFromConfig(); got != "auto" {
		t.Errorf("SummarizerFromConfig()=%q, want auto (do config.json)", got)
	}
	// env tem prioridade sobre o config.json.
	t.Setenv("AGENT_SYNC_SUMMARIZER", "ollama:qwen2.5:3b")
	if got := SummarizerFromConfig(); got != "ollama:qwen2.5:3b" {
		t.Errorf("SummarizerFromConfig()=%q, want ollama:qwen2.5:3b (env vence)", got)
	}
}

func TestLatestSessionForProject(t *testing.T) {
	withTempCache(t)
	proj := "/tmp/test-project"
	s1, err := Load("sess-1")
	if err != nil {
		t.Fatal(err)
	}
	s1.ProjectPath = proj
	if err := s1.Save(); err != nil {
		t.Fatal(err)
	}

	found, err := LatestSessionForProject(proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != "sess-1" {
		t.Fatalf("expected sess-1, got %s", found.ID)
	}
}

func TestSummarizeActiveHypothesesAddsBlockerToNextSteps(t *testing.T) {
	inputYAML := `decisions:
  - "Usar SQLite local"
active_hypotheses:
  - "(hypothesis) deadlock ocorre na liberação de lock"
next_steps:
  - "Rodar benchmark de concorrência"
`
	got := ensureHypothesesGuard(inputYAML)

	if !strings.Contains(got, activeHypothesesBlockerText) {
		t.Fatalf("expected next_steps to contain blocker %q, got:\n%s", activeHypothesesBlockerText, got)
	}

	expectedLine := `- "` + activeHypothesesBlockerText + `"`
	if !strings.Contains(got, expectedLine) {
		t.Errorf("expected output to contain line %q, got:\n%s", expectedLine, got)
	}

	if !strings.Contains(got, "Rodar benchmark de concorrência") {
		t.Errorf("expected original next step to be preserved, got:\n%s", got)
	}

	records := agentmemory.ParseSummary(got)
	foundBlocker := false
	for _, r := range records {
		if r.Section == "next_steps" && strings.Contains(r.Content, activeHypothesesBlockerText) {
			foundBlocker = true
			break
		}
	}
	if !foundBlocker {
		t.Errorf("ParseSummary did not find blocker in next_steps records")
	}
}

func TestSummarizeActiveHypothesesDoesNotDuplicateBlocker(t *testing.T) {
	t.Run("already has quoted blocker", func(t *testing.T) {
		inputYAML := `decisions:
  - "Decisão 1"
active_hypotheses:
  - "(hypothesis) bug no parser"
next_steps:
  - "` + activeHypothesesBlockerText + `"
  - "Passo subsequente"
`
		got := ensureHypothesesGuard(inputYAML)
		count := strings.Count(got, activeHypothesesBlockerText)
		if count != 1 {
			t.Fatalf("expected exactly 1 blocker occurrence, got %d. Output:\n%s", count, got)
		}
		if got != inputYAML {
			t.Errorf("expected input to remain unchanged, got:\n%s", got)
		}
	})

	t.Run("already has unquoted blocker", func(t *testing.T) {
		inputYAML := `active_hypotheses:
  - "hipotese 1"
next_steps:
  - ` + activeHypothesesBlockerText + `
  - "Passo 2"
`
		got := ensureHypothesesGuard(inputYAML)
		count := strings.Count(got, activeHypothesesBlockerText)
		if count != 1 {
			t.Fatalf("expected exactly 1 blocker occurrence, got %d. Output:\n%s", count, got)
		}
	})

	t.Run("idempotent when called multiple times", func(t *testing.T) {
		initialYAML := `active_hypotheses:
  - "hipotese pendente"
next_steps:
  - "Passo 1"
`
		first := ensureHypothesesGuard(initialYAML)
		second := ensureHypothesesGuard(first)
		third := ensureHypothesesGuard(second)

		count := strings.Count(third, activeHypothesesBlockerText)
		if count != 1 {
			t.Fatalf("expected exactly 1 blocker after multiple passes, got %d. Output:\n%s", count, third)
		}
		if second != first || third != second {
			t.Errorf("expected idempotent transformation across multiple runs")
		}
	})
}

func TestSummarizeNoActiveHypothesesDoesNotAddBlocker(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "inline empty list",
			yaml: `decisions:
  - "D1"
active_hypotheses: []
next_steps:
  - "Passo 1"
`,
		},
		{
			name: "multiline empty list",
			yaml: `decisions:
  - "D1"
active_hypotheses:
  []
next_steps:
  - "Passo 1"
`,
		},
		{
			name: "empty section key",
			yaml: `decisions:
  - "D1"
active_hypotheses:
next_steps:
  - "Passo 1"
`,
		},
		{
			name: "missing active_hypotheses section",
			yaml: `decisions:
  - "D1"
next_steps:
  - "Passo 1"
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ensureHypothesesGuard(tc.yaml)
			if strings.Contains(got, activeHypothesesBlockerText) {
				t.Fatalf("expected no blocker added when active_hypotheses is empty, got:\n%s", got)
			}
		})
	}
}

func TestSummarizeActiveHypothesesEmptyOrMissingNextSteps(t *testing.T) {
	t.Run("inline empty next_steps", func(t *testing.T) {
		input := `active_hypotheses:
  - "hipotese 1"
next_steps: []
constraints:
  - "C1"
`
		got := ensureHypothesesGuard(input)
		if !strings.Contains(got, activeHypothesesBlockerText) {
			t.Fatalf("expected blocker added to inline empty next_steps, got:\n%s", got)
		}
		if !strings.Contains(got, "constraints:") {
			t.Errorf("expected constraints to be preserved, got:\n%s", got)
		}
	})

	t.Run("multiline empty next_steps", func(t *testing.T) {
		input := `active_hypotheses:
  - "hipotese 1"
next_steps:
  []
constraints:
  - "C1"
`
		got := ensureHypothesesGuard(input)
		if !strings.Contains(got, activeHypothesesBlockerText) {
			t.Fatalf("expected blocker added to multiline empty next_steps, got:\n%s", got)
		}
	})

	t.Run("missing next_steps section", func(t *testing.T) {
		input := `decisions:
  - "D1"
active_hypotheses:
  - "hipotese 1"
`
		got := ensureHypothesesGuard(input)
		if !strings.Contains(got, "next_steps:") {
			t.Fatalf("expected next_steps section to be created, got:\n%s", got)
		}
		if !strings.Contains(got, activeHypothesesBlockerText) {
			t.Fatalf("expected blocker in created next_steps, got:\n%s", got)
		}
	})
}

func TestSummarizeMultipleActiveHypothesesAddsSingleBlocker(t *testing.T) {
	input := `active_hypotheses:
  - "hipotese 1"
  - "hipotese 2"
  - "hipotese 3"
next_steps:
  - "Passo 1"
`
	got := ensureHypothesesGuard(input)
	count := strings.Count(got, activeHypothesesBlockerText)
	if count != 1 {
		t.Fatalf("expected exactly 1 blocker occurrence for multiple hypotheses, got %d. Output:\n%s", count, got)
	}
}
