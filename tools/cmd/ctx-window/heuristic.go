package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ExtractedSummary is the result of the local heuristic summarizer.
// Extracts decision, hypothesis, artifact, error, next-step, and constraint
// markers from the last tool calls. It is the guaranteed fallback (no
// external LLM) — lower quality than the own-model summarizer, but it
// never fails.
type ExtractedSummary struct {
	Decisoes       []string
	Hipoteses      []string
	Artefatos      []string
	Erros          []string
	ProximosPassos []string
	Restricoes     []string
}

// Markers are bilingual (PT-BR + EN) so the fallback works regardless of
// the agent's operating language. Order matters: more specific patterns
// first to avoid false matches.
var (
	decisionMarkers    = regexp.MustCompile(`(?i)\b(decidimos|vamos usar|foi definido|vou adotar|concordamos|optamos por|escolhemos|we decided|we will use|decided to|we agreed|let's use|agreed to|was defined|was set to|chose to)\b`)
	hypothesisMarkers  = regexp.MustCompile(`(?i)\b(hip[oó]tese|vou testar|se .+ ent[aã]o .+|ser[aá] que|hypothesis|i will test|we will test|i'll test|let's test|assuming|if .+ then .+)\b`)
	errorMarkers       = regexp.MustCompile(`(?i)\b(erro|falha|exception|panic|n[ãa]o funciona|quebrou|crash|error|failure|broken|failed to)\b`)
	stepMarkers        = regexp.MustCompile(`(?i)\b(pr[oó]ximo passo|todo|falta|pendente|a fazer|depois|next step|todo:|pending|to do|later)\b`)
	restrictionMarkers = regexp.MustCompile(`(?i)\b(n[aã]o podemos|proibido|regra:|nunca |sempre |obrigat[oó]rio|cannot|forbidden|rule:|never |always |mandatory|must not)\b`)
	artifactPattern    = regexp.MustCompile(`(?P<path>[a-zA-Z0-9_./-]+\.[a-zA-Z0-9]+(?::[0-9]+)?)`)
)

// HeuristicExtract applies the markers over the turns and returns a
// deduplicated ExtractedSummary (case-insensitive on texts).
func HeuristicExtract(turns []Turn) ExtractedSummary {
	var s ExtractedSummary
	decSeen := map[string]bool{}
	hipSeen := map[string]bool{}
	artSeen := map[string]bool{}
	errSeen := map[string]bool{}
	stepSeen := map[string]bool{}
	resSeen := map[string]bool{}

	for _, t := range turns {
		text := t.Content
		if text == "" {
			continue
		}
		for _, sent := range splitSentences(text) {
			sent = strings.TrimSpace(sent)
			if sent == "" {
				continue
			}
			switch {
			case decisionMarkers.MatchString(sent):
				addUnique(&s.Decisoes, decSeen, sent)
			case restrictionMarkers.MatchString(sent):
				addUnique(&s.Restricoes, resSeen, sent)
			case hypothesisMarkers.MatchString(sent):
				addUnique(&s.Hipoteses, hipSeen, sent)
			case errorMarkers.MatchString(sent):
				addUnique(&s.Erros, errSeen, sent)
			case stepMarkers.MatchString(sent):
				addUnique(&s.ProximosPassos, stepSeen, sent)
			}
		}
		for _, m := range artifactPattern.FindAllString(text, -1) {
			if looksLikePath(m) {
				addUnique(&s.Artefatos, artSeen, m)
			}
		}
	}
	// sort for deterministic output
	sort.Strings(s.Decisoes)
	sort.Strings(s.Hipoteses)
	sort.Strings(s.Artefatos)
	sort.Strings(s.Erros)
	sort.Strings(s.ProximosPassos)
	sort.Strings(s.Restricoes)
	return s
}

// ToYAML serializes the summary in the format expected by the prompt
// at skills/context-window-strategy/prompts/summarize.md.
func (s ExtractedSummary) ToYAML() string {
	var b strings.Builder
	b.WriteString("decisions:\n")
	writeYAMLList(&b, s.Decisoes)
	b.WriteString("active_hypotheses:\n")
	writeYAMLList(&b, s.Hipoteses)
	b.WriteString("artifacts:\n")
	if len(s.Artefatos) == 0 {
		b.WriteString("  []\n")
	} else {
		for _, a := range s.Artefatos {
			fmt.Fprintf(&b, "  - path: %q\n", a)
			b.WriteString("    description: \"\"\n")
		}
	}
	b.WriteString("resolved_errors:\n")
	writeYAMLList(&b, s.Erros)
	b.WriteString("next_steps:\n")
	writeYAMLList(&b, s.ProximosPassos)
	b.WriteString("constraints:\n")
	writeYAMLList(&b, s.Restricoes)
	return b.String()
}

func writeYAMLList(b *strings.Builder, items []string) {
	if len(items) == 0 {
		b.WriteString("  []\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "  - %q\n", item)
	}
}

func addUnique(dst *[]string, seen map[string]bool, item string) {
	key := strings.ToLower(strings.TrimSpace(item))
	if seen[key] {
		return
	}
	seen[key] = true
	*dst = append(*dst, item)
}

func splitSentences(text string) []string {
	// rough separators sufficient for marker extraction
	re := regexp.MustCompile(`[\n\.\!\?\;…]+`)
	parts := re.Split(text, -1)
	return parts
}

func looksLikePath(s string) bool {
	// ignore http URLs and clearly path-less sequences
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return false
	}
	if !strings.Contains(s, ".") {
		return false
	}
	// must have path separator, path:line format, or a known extension
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return true
	}
	if strings.Contains(s, ":") {
		// format "file.ext:line" — accept if the prefix ends with a known ext
		prefix := s[:strings.Index(s, ":")]
		knownExt := []string{".go", ".py", ".ts", ".js", ".sh", ".md", ".json", ".yaml", ".yml", ".toml", ".sql", ".html", ".css"}
		for _, ext := range knownExt {
			if strings.HasSuffix(prefix, ext) {
				return true
			}
		}
	}
	knownExt := []string{".go", ".py", ".ts", ".js", ".sh", ".md", ".json", ".yaml", ".yml", ".toml", ".sql", ".html", ".css"}
	for _, ext := range knownExt {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}
