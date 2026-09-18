package main

import (
	"context"
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
