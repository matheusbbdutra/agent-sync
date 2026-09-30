package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Activity é o resultado do ClassifyActivity: categoria dominante + score
// normalizado (0-100) + distribuição por categoria.
type Activity struct {
	Categories []ActivityCount `json:"categories"` // ordenado por count desc
	Mode       string          `json:"mode"`       // categoria dominante
	Score      int             `json:"score"`      // 0-100, % da categoria top
	Window     int             `json:"window"`     // nº de turns efetivamente analisados
}

// ActivityCount é uma categoria + nº de turns classificados nela.
type ActivityCount struct {
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

const activityWindowSize = 10

// classWeights mapeia (categoria, substr do tool name) → peso. Maior peso
// = sinal mais forte daquela categoria. Substr em lowercase.
var classWeights = []struct {
	Mode  string
	Match string
	Score int
}{
	{"code", "edit", 3},
	{"code", "write", 3},
	{"code", "notebookedit", 3},
	{"debug", "bash", 1},
	{"debug", "grep", 1},
	{"review", "read", 2},
	{"review", "glob", 1},
	{"infra", "bash", 1}, // infra é decidido por keyword do content, não tool
	{"general", "agent", 1},
	{"general", "task", 1},
}

// keywordBoost adiciona bônus quando o conteúdo (lowercase) contém uma
// keyword típica da categoria. Independente do tool — serve para
// diferenciar "Bash para debug" de "Bash para infra".
var keywordBoost = []struct {
	Mode    string
	Keyword string
	Bonus   int
}{
	{"debug", "error", 2},
	{"debug", "fail", 2},
	{"debug", "panic", 3},
	{"debug", "traceback", 3},
	{"debug", "stack trace", 3},
	{"debug", "test", 1},
	{"infra", "kubectl", 4},
	{"infra", "terraform", 4},
	{"infra", "docker", 3},
	{"infra", "curl", 2},
	{"infra", "ssh", 2},
	{"infra", "deploy", 3},
	{"code", "implement", 2},
	{"code", "refactor", 2},
	{"code", "fix", 1},
	{"review", "review", 2},
	{"review", "audit", 2},
	{"review", "explain", 1},
}

// ClassifyActivity devolve a categoria dominante das últimas N tool calls.
// Inspirado no `activity_tracker.py` do token-optimizer externo: zero
// chamada LLM, contagem pura baseada em tool+keywords. Função pura.
func ClassifyActivity(turns []Turn) Activity {
	n := len(turns)
	if n == 0 {
		return Activity{Mode: "general", Score: 0, Window: 0}
	}
	start := n - activityWindowSize
	if start < 0 {
		start = 0
	}
	scores := make(map[string]int, 6)
	for i := start; i < n; i++ {
		tool, content := splitTurn(turns[i].Content)
		toolLower := strings.ToLower(tool)
		contentLower := strings.ToLower(content)
		for _, w := range classWeights {
			if strings.Contains(toolLower, w.Match) {
				scores[w.Mode] += w.Score
			}
		}
		for _, kw := range keywordBoost {
			if strings.Contains(contentLower, kw.Keyword) {
				scores[kw.Mode] += kw.Bonus
			}
		}
	}
	if len(scores) == 0 {
		return Activity{Mode: "general", Score: 0, Window: n - start}
	}
	total := 0
	for _, v := range scores {
		total += v
	}
	// ordena por score desc; desempate por nome lexicográfico para estabilidade
	var cats []ActivityCount
	for mode, score := range scores {
		cats = append(cats, ActivityCount{Mode: mode, Count: score})
	}
	sortActivityCounts(cats)
	top := cats[0]
	mode := top.Mode
	if top.Count == 0 {
		mode = "general"
	}
	pct := 0
	if total > 0 {
		pct = top.Count * 100 / total
	}
	return Activity{
		Categories: cats,
		Mode:       mode,
		Score:      pct,
		Window:     n - start,
	}
}

func sortActivityCounts(cats []ActivityCount) {
	// insertion sort (n pequeno)
	for i := 1; i < len(cats); i++ {
		for j := i; j > 0 && (cats[j-1].Count < cats[j].Count ||
			(cats[j-1].Count == cats[j].Count && cats[j-1].Mode > cats[j].Mode)); j-- {
			cats[j-1], cats[j] = cats[j], cats[j-1]
		}
	}
}

// splitTurn separa o tool name do resto do conteúdo. AddTurn grava
// "tool: content" (ver runOnToolCall em main.go); então a primeira
// palavra antes do primeiro ":" é o tool.
func splitTurn(content string) (tool, rest string) {
	idx := strings.Index(content, ":")
	if idx < 1 {
		return strings.TrimSpace(content), ""
	}
	return strings.TrimSpace(content[:idx]), strings.TrimSpace(content[idx+1:])
}

// Write imprime Activity em formato humano (default) ou JSON.
func (a *Activity) Write(out io.Writer, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(a)
	}
	fmt.Fprintf(out, "activity for last %d tool calls\n", a.Window)
	fmt.Fprintf(out, "  mode:  %s\n", a.Mode)
	fmt.Fprintf(out, "  score: %d/100\n\n", a.Score)
	fmt.Fprintln(out, "  distribution:")
	for _, c := range a.Categories {
		fmt.Fprintf(out, "    %-10s  %d\n", c.Mode, c.Count)
	}
	return nil
}
