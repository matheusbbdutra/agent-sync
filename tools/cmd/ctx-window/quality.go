package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// QualitySignal é um dos 7 sinais que compõem o score agregado da sessão.
// Inspirado nos 7 sinais do token-optimizer (alexgreensh), mas calculado
// puramente sobre dados já disponíveis no ctx-window (turns.jsonl +
// meta.json + summary.md) — zero dependência externa.
type QualitySignal struct {
	Name   string `json:"name"`   // chave estável: context_fill | compaction_depth | ...
	Score  int    `json:"score"`  // 0-100 (100 = ideal)
	Weight int    `json:"weight"` // % de contribuição no score final (0-100)
	Reason string `json:"reason"` // diagnóstico de 1 linha (human-readable)
}

// QualityReport agrega os 7 sinais + score final (média ponderada) + letra.
//
// Faixas: S ≥ 90, A ≥ 80, B ≥ 70, C ≥ 55, D ≥ 40, F < 40.
// Pesos default: 25/15/15/15/10/10/10 = 100.
type QualityReport struct {
	SessionID  string          `json:"session_id"`
	Signals    []QualitySignal `json:"signals"`
	Score      int             `json:"score"` // 0-100
	Grade      string          `json:"grade"` // S/A/B/C/D/F
	ComputedAt time.Time       `json:"computed_at"`
}

const (
	qWeightContextFill       = 25
	qWeightCompactionDepth   = 15
	qWeightWasteTokens       = 15
	qWeightStaleReads        = 15
	qWeightBloatedResults    = 10
	qWeightDecisionDensity   = 10
	qWeightAgentEfficiency   = 10
	qThresholdBloatedChars   = 4000 // turn acima disso conta como bloated
	qThresholdStaleRepeatRun = 2    // count de repetições p/ sinal stale
)

// ComputeQuality calcula o QualityReport de uma sessão. Função pura:
// só lê do disco (summary.md) o que Session já sabe referenciar. Sem
// chamadas de LLM, sem subprocessos.
func ComputeQuality(s *Session) QualityReport {
	signals := []QualitySignal{
		signalContextFill(s),
		signalCompactionDepth(s),
		signalWasteTokens(s),
		signalStaleReads(s),
		signalBloatedResults(s),
		signalDecisionDensity(s),
		signalAgentEfficiency(s),
	}
	var totalWeight, weighted int
	for _, sig := range signals {
		if sig.Weight <= 0 {
			continue
		}
		totalWeight += sig.Weight
		weighted += sig.Score * sig.Weight
	}
	score := 0
	if totalWeight > 0 {
		score = weighted / totalWeight
	}
	return QualityReport{
		SessionID:  s.ID,
		Signals:    signals,
		Score:      clampScore(score),
		Grade:      GradeFromScore(score),
		ComputedAt: time.Now().UTC(),
	}
}

// GradeFromScore devolve a letra S/A/B/C/D/F conforme faixas
// alinhadas ao paper do token-optimizer externo.
func GradeFromScore(score int) string {
	switch {
	case score >= 90:
		return "S"
	case score >= 80:
		return "A"
	case score >= 70:
		return "B"
	case score >= 55:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// signalContextFill: ideal < 50% do threshold; ruim ≥ 100%.
// score 100 → fill=0; cai linear até 0 quando fill ≥ 2*threshold.
func signalContextFill(s *Session) QualitySignal {
	fill := s.EstimatedChars()
	threshold := compactAtThreshold()
	score := 100
	reason := fmt.Sprintf("%d/%d chars", fill, threshold)
	if threshold > 0 {
		ratio := float64(fill) / float64(threshold)
		score = clampScore(int(100 - ratio*50))
		if ratio > 1 {
			reason = fmt.Sprintf("%d/%d chars (over threshold)", fill, threshold)
		}
	}
	return QualitySignal{
		Name:   "context_fill",
		Score:  score,
		Weight: qWeightContextFill,
		Reason: reason,
	}
}

// signalCompactionDepth: 0 compactações = ideal; ≥ 5 = ruim.
// score 100 → 0 compactions; cai 20 por compactação até 0.
func signalCompactionDepth(s *Session) QualitySignal {
	v := s.Version
	score := clampScore(100 - v*20)
	reason := fmt.Sprintf("%d compactions", v)
	if v >= 5 {
		reason = fmt.Sprintf("%d compactions (deep)", v)
	}
	return QualitySignal{
		Name:   "compaction_depth",
		Score:  score,
		Weight: qWeightCompactionDepth,
		Reason: reason,
	}
}

// signalWasteTokens: chars acima do threshold = "waste".
// score 100 → waste ≤ threshold; 0 → waste ≥ 3*threshold.
func signalWasteTokens(s *Session) QualitySignal {
	fill := s.EstimatedChars()
	threshold := compactAtThreshold()
	waste := fill - threshold
	if waste < 0 {
		waste = 0
	}
	score := 100
	reason := fmt.Sprintf("%d chars over threshold", waste)
	if threshold > 0 {
		ratio := float64(waste) / float64(3*threshold)
		score = clampScore(int(100 - ratio*100))
	}
	return QualitySignal{
		Name:   "waste_tokens",
		Score:  score,
		Weight: qWeightWasteTokens,
		Reason: reason,
	}
}

// signalStaleReads: % de turns repetidos (DetectLoop já calculou a chave).
// score 100 → 0 repetições; 0 → ≥ 50% dos turns são dups na janela.
func signalStaleReads(s *Session) QualitySignal {
	if len(s.Turns) == 0 {
		return QualitySignal{Name: "stale_reads", Score: 100, Weight: qWeightStaleReads, Reason: "no turns"}
	}
	counts := make(map[string]int, len(s.Turns))
	dups := 0
	for _, t := range s.Turns {
		k := loopKey(t.Content)
		if k == "" {
			continue
		}
		counts[k]++
		if counts[k] > qThresholdStaleRepeatRun {
			dups++
		}
	}
	ratio := float64(dups) / float64(len(s.Turns))
	score := clampScore(int(100 - ratio*200))
	return QualitySignal{
		Name:   "stale_reads",
		Score:  score,
		Weight: qWeightStaleReads,
		Reason: fmt.Sprintf("%d duplicate turns", dups),
	}
}

// signalBloatedResults: % de turns com content > 4KB (suspeito de bloat).
func signalBloatedResults(s *Session) QualitySignal {
	if len(s.Turns) == 0 {
		return QualitySignal{Name: "bloated_results", Score: 100, Weight: qWeightBloatedResults, Reason: "no turns"}
	}
	bloated := 0
	for _, t := range s.Turns {
		if len(t.Content) > qThresholdBloatedChars {
			bloated++
		}
	}
	ratio := float64(bloated) / float64(len(s.Turns))
	score := clampScore(int(100 - ratio*200))
	return QualitySignal{
		Name:   "bloated_results",
		Score:  score,
		Weight: qWeightBloatedResults,
		Reason: fmt.Sprintf("%d/%d turns > %d chars", bloated, len(s.Turns), qThresholdBloatedChars),
	}
}

// signalDecisionDensity: decisions por turn, baseado no summary.md atual.
// Fallback 0 quando não há summary — não penaliza sessões novas.
func signalDecisionDensity(s *Session) QualitySignal {
	dir, err := SessionDir(s.ID)
	if err != nil {
		return QualitySignal{Name: "decision_density", Score: 0, Weight: qWeightDecisionDensity, Reason: "session dir unavailable"}
	}
	body, err := os.ReadFile(summaryPath(dir))
	if err != nil {
		return QualitySignal{Name: "decision_density", Score: 0, Weight: qWeightDecisionDensity, Reason: "no summary yet"}
	}
	decisions := countDecisionsInSummary(string(body))
	if decisions == 0 {
		return QualitySignal{Name: "decision_density", Score: 0, Weight: qWeightDecisionDensity, Reason: "0 decisions in summary"}
	}
	perTurn := float64(decisions) / float64(len(s.Turns))
	// ideal: ≥ 0.5 decisions/turn; ruim: 0
	score := clampScore(int(perTurn * 200))
	return QualitySignal{
		Name:   "decision_density",
		Score:  score,
		Weight: qWeightDecisionDensity,
		Reason: fmt.Sprintf("%d decisions / %d turns", decisions, len(s.Turns)),
	}
}

// signalAgentEfficiency: turns por compactação. Muito turn sem compactação
// = janela inchando; 0 = sessão nova (neutro).
func signalAgentEfficiency(s *Session) QualitySignal {
	if s.Version == 0 {
		return QualitySignal{Name: "agent_efficiency", Score: 100, Weight: qWeightAgentEfficiency, Reason: "no compactions yet"}
	}
	perCompact := float64(len(s.Turns)) / float64(s.Version)
	// ideal: ≤ 2 turns/compact; ruim: ≥ 10
	score := clampScore(int(100 - (perCompact-2)*15))
	return QualitySignal{
		Name:   "agent_efficiency",
		Score:  score,
		Weight: qWeightAgentEfficiency,
		Reason: fmt.Sprintf("%.1f turns per compaction", perCompact),
	}
}

// countDecisionsInSummary parseia o YAML mínimo produzido por
// ExtractedSummary.ToYAML() e conta itens sob a chave "decisions:".
// Tolerante a comentários e whitespace; falha silenciosa → 0.
func countDecisionsInSummary(yamlText string) int {
	inDecisions := false
	count := 0
	for _, line := range strings.Split(yamlText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "decisions:") {
			inDecisions = true
			continue
		}
		if !inDecisions {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			count++
			continue
		}
		// outra seção top-level (active_hypotheses:, artifacts:, ...)
		if !strings.HasPrefix(trimmed, "-") && strings.Contains(trimmed, ":") {
			inDecisions = false
		}
	}
	return count
}

// WriteQualityReport imprime o QualityReport em formato humano
// (default) ou JSON (--json).
func (r *QualityReport) Write(w io.Writer, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Fprintf(w, "Quality report for session %q\n", r.SessionID)
	fmt.Fprintf(w, "  score: %d/100  grade: %s\n\n", r.Score, r.Grade)
	for _, sig := range r.Signals {
		fmt.Fprintf(w, "  [%s] %3d (w=%d%%)  %s\n", sig.Name, sig.Score, sig.Weight, sig.Reason)
	}
	return nil
}
