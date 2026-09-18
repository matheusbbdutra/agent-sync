package main

import (
	"context"
	"strings"
	"testing"
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
