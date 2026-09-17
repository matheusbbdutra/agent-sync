package main

import (
	"strings"
	"testing"
	"time"
)

func turnAt(role, content string, ago time.Duration) Turn {
	return Turn{Role: role, Content: content, At: time.Now().UTC().Add(-ago)}
}

func TestHeuristicExtractDecisions(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "We decided to use sliding window with K=5.", 3*time.Minute),
		turnAt("user", "ok, let's use local heuristic as fallback.", 2*time.Minute),
		turnAt("assistant", "Token budget was defined as 1000.", 1*time.Minute),
	}
	s := HeuristicExtract(turns)
	if len(s.Decisoes) != 3 {
		t.Fatalf("expected 3 decisions, got %d: %v", len(s.Decisoes), s.Decisoes)
	}
}

func TestHeuristicExtractHypotheses(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "Hypothesis: local heuristic may reach 80%.", 1*time.Minute),
		turnAt("assistant", "I will test with mini-projects.", 30*time.Second),
	}
	s := HeuristicExtract(turns)
	if len(s.Hipoteses) == 0 {
		t.Fatal("expected at least 1 hypothesis, got 0")
	}
}

func TestHeuristicExtractArtifacts(t *testing.T) {
	turns := []Turn{
		turnAt("tool", "touched tools/cmd/ctx-window/main.go:42 and skills/context-window-strategy/SKILL.md:1", 1*time.Minute),
	}
	s := HeuristicExtract(turns)
	if len(s.Artefatos) < 2 {
		t.Fatalf("expected 2 artifacts, got %d: %v", len(s.Artefatos), s.Artefatos)
	}
	for _, a := range s.Artefatos {
		if strings.HasPrefix(a, "http") {
			t.Fatalf("artifact should not contain URL: %q", a)
		}
	}
}

func TestHeuristicExtractErrors(t *testing.T) {
	turns := []Turn{
		turnAt("tool", "Error: hook failed with exit 127. We will investigate.", 1*time.Minute),
		turnAt("assistant", "panic: nil pointer dereference.", 30*time.Second),
	}
	s := HeuristicExtract(turns)
	if len(s.Erros) < 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(s.Erros), s.Erros)
	}
}

func TestHeuristicExtractNextSteps(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "Next step: implement ctx-compact hook.", 1*time.Minute),
		turnAt("assistant", "TODO: define default Ollama model.", 30*time.Second),
	}
	s := HeuristicExtract(turns)
	if len(s.ProximosPassos) < 2 {
		t.Fatalf("expected 2 next steps, got %d: %v", len(s.ProximosPassos), s.ProximosPassos)
	}
}

func TestHeuristicExtractConstraints(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "We cannot use external LLM; privacy is mandatory.", 1*time.Minute),
	}
	s := HeuristicExtract(turns)
	if len(s.Restricoes) == 0 {
		t.Fatal("expected 1 constraint, got 0")
	}
}

func TestHeuristicExtractDedupes(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "We decided to use K=5.", 1*time.Minute),
		turnAt("assistant", "we decided to use K=5.", 30*time.Second),
		turnAt("assistant", "WE DECIDED TO USE K=5.", 10*time.Second),
	}
	s := HeuristicExtract(turns)
	if len(s.Decisoes) != 1 {
		t.Fatalf("expected 1 deduplicated decision, got %d: %v", len(s.Decisoes), s.Decisoes)
	}
}

func TestHeuristicExtractIgnoresEmptySentences(t *testing.T) {
	turns := []Turn{
		turnAt("assistant", "   \n\n  ", 1*time.Minute),
		turnAt("assistant", "", 30*time.Second),
	}
	s := HeuristicExtract(turns)
	if len(s.Decisoes)+len(s.Hipoteses)+len(s.Artefatos)+len(s.Erros)+len(s.ProximosPassos)+len(s.Restricoes) != 0 {
		t.Fatalf("expected empty summary, got %+v", s)
	}
}

func TestHeuristicAcceptsPTBRMarkers(t *testing.T) {
	// keep PT-BR coverage so the fallback works for agents operating in PT-BR
	turns := []Turn{
		turnAt("assistant", "Decidimos usar K=5. Vou testar com heurística. Próximo passo: hook.", 1*time.Minute),
	}
	s := HeuristicExtract(turns)
	if len(s.Decisoes) == 0 || len(s.ProximosPassos) == 0 {
		t.Fatalf("PT-BR markers should still be recognized, got %+v", s)
	}
}

func TestExtractedSummaryToYAML(t *testing.T) {
	s := ExtractedSummary{
		Decisoes:  []string{"use K=5"},
		Hipoteses: []string{"heuristic reaches 80%"},
		Artefatos: []string{"tools/cmd/ctx-window/main.go"},
		Erros:     []string{"hook failed"},
	}
	yaml := s.ToYAML()
	for _, key := range []string{"decisions:", "active_hypotheses:", "artifacts:", "resolved_errors:", "next_steps:", "constraints:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("YAML missing section %q", key)
		}
	}
}

func TestExtractedSummaryYAMLEmpty(t *testing.T) {
	s := ExtractedSummary{}
	yaml := s.ToYAML()
	for _, key := range []string{"decisions:", "active_hypotheses:", "artifacts:", "resolved_errors:", "next_steps:", "constraints:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("empty YAML missing section %q", key)
		}
	}
}

func TestLooksLikePath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"tools/cmd/ctx-window/main.go", true},
		{"main.go:42", true},
		{"https://example.com", false},
		{"plainword", false},
		{"a.json", true},
		{"README", false},
	}
	for _, c := range cases {
		if got := looksLikePath(c.in); got != c.want {
			t.Errorf("looksLikePath(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
