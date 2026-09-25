// budget_status.go: token budget status (utilization, threshold, nudge).
//
// token_budget.go implementa o subcommand `agent-sync budget nudge` que
// produz o payload `token-budget-status.json` consumido pelos hooks
// cross-CLI (postToolUse de Claude Code/Codex/Antigravity/Cursor e plugin
// OpenCode v2). Migrado de token_budget.go em 2026-09-21 (Fase 5).
// Renomeado: token_budget.go -> budget_status.go (prefir prefixo budget_

// token_budget.go implementa o subcommand `agent-sync budget nudge` que
// produz o payload `token-budget-status.json` consumido pelos hooks
// cross-CLI (postToolUse de Claude Code/Codex/Antigravity/Cursor e plugin
// v2 do OpenCode). Logica de decisao conforme ADR-token-nudge-contract
// Decisao 3: should_nudge = (utilization_pct >= threshold_pct) OR
// (heuristica legada dispara).
//
// Heuristica legada (rede de seguranca para gaps Antigravity/Cursor):
//   - tool_calls >= AGENT_SYNC_CTX_WINDOW_NUDGE_MIN_TOOL_CALLS (default 80)
//   - summary.md ausente
//   - summary.md com idade >= AGENT_SYNC_CTX_WINDOW_NUDGE_MAX_AGE_HOURS (default 4h)
package budget

// budget_status.go: token budget status (utilization, threshold, nudge).
//
// Usado por hooks token-nudge cross-CLI (A-16) e OpenCode v2 plugin.
// Migrado de token_budget.go em 2026-09-21 (Fase 5). Renomeado:
// token_budget.go -> budget_status.go (prefir prefixo budget_).
import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/agent-sync/internal/state"
	"github.com/matheusdutra/token-tools/jsonschema"
)

const (
	TokenBudgetStatusSchemaVersion = "1.0"
	TokenNudgeDefaultThresholdPct  = 80
	TokenNudgeDefaultMinToolCalls  = 80
	TokenNudgeDefaultMaxAgeHours   = 4
)

// ContextWindowLookup retorna o context window maximo para um modelo.
// Tabela minima: claude-sonnet-4.5 (200k), claude-opus-4.6 (200k),
// gpt-5 (400k), gpt-5-mini (400k), gemini-2.5-pro (1M). Default 200k
// quando modelo desconhecido (conservador: subestima janela, nudge
// dispara mais cedo).
func ContextWindowLookup(model string) int {
	switch model {
	case "claude-sonnet-4.5", "claude-sonnet-4.6", "claude-opus-4.6", "claude-opus-4.7", "claude-haiku-4.5":
		return 200000
	case "gpt-5", "gpt-5-mini", "gpt-5-codex", "gpt-5.1":
		return 400000
	case "gemini-2.5-pro", "gemini-2.5-flash":
		return 1000000
	default:
		return 200000
	}
}

// TokenBudgetStatus e o payload canonico do schema token-budget-status.json.
type TokenBudgetStatus struct {
	SchemaVersion string                 `json:"schema_version"`
	TS            time.Time              `json:"ts"`
	Actor         string                 `json:"actor"`
	SessionID     string                 `json:"session_id"`
	Model         string                 `json:"model,omitempty"`
	TokensIn      int                    `json:"tokens_in,omitempty"`
	TokensOut     int                    `json:"tokens_out,omitempty"`
	TokensTotal   int                    `json:"tokens_total"`
	ContextWindow int                    `json:"context_window,omitempty"`
	UtilizationPct int                   `json:"utilization_pct"`
	ThresholdPct  int                    `json:"threshold_pct"`
	ShouldNudge   bool                   `json:"should_nudge"`
	Trigger       string                 `json:"trigger"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// BuildTokenBudgetStatus monta o status a partir das informacoes disponiveis.
// `actor` e a CLI origem; `tokensIn/Out` vir da fonte especifica (bash hook
// le transcript; OpenCode v2 vem do SDK); `model` opcional; `sessionID`
// obrigatorio; `summaryPath` e `toolCalls` sao insumos da heuristica
// legada.
func BuildTokenBudgetStatus(actor, sessionID, model string, tokensIn, tokensOut, toolCalls int, summaryPath string, thresholdPct int, details map[string]interface{}) (TokenBudgetStatus, error) {
	if actor == "" {
		actor = "agent-sync"
	}
	if sessionID == "" {
		sessionID = "unknown"
	}
	if thresholdPct <= 0 {
		thresholdPct = TokenNudgeDefaultThresholdPct
	}
	now := time.Now().UTC()
	total := tokensIn + tokensOut
	cw := 0
	utilPct := 0
	// Se model ausente, usa default conservador (Claude Sonnet 4.5 = 200k).
	// Garante utilization_pct > 0 quando tokens existem, mesmo sem model
	// nativo (Claude/Antigravity/Cursor nao trazem model no payload).
	effectiveModel := model
	if effectiveModel == "" {
		effectiveModel = "default"
	}
	cw = ContextWindowLookup(effectiveModel)
	if cw > 0 && total > 0 {
		utilPct = (total * 100) / cw
		if utilPct > 100 {
			utilPct = 100
		}
	}
	should, trigger := decideNudge(total, utilPct, thresholdPct, toolCalls, summaryPath)
	s := TokenBudgetStatus{
		SchemaVersion:  TokenBudgetStatusSchemaVersion,
		TS:             now,
		Actor:          actor,
		SessionID:      sessionID,
		Model:          model,
		TokensIn:       tokensIn,
		TokensOut:      tokensOut,
		TokensTotal:    total,
		ContextWindow:  cw,
		UtilizationPct: utilPct,
		ThresholdPct:   thresholdPct,
		ShouldNudge:    should,
		Trigger:        trigger,
		Details:        details,
	}
	if err := jsonschema.Validate("token-budget-status", s); err != nil {
		return TokenBudgetStatus{}, fmt.Errorf("token-budget-status: schema invalido: %w", err)
	}
	return s, nil
}

func decideNudge(total, utilPct, thresholdPct, toolCalls int, summaryPath string) (should bool, trigger string) {
	if utilPct >= thresholdPct && total > 0 {
		return true, "tokens_in>=threshold"
	}
	if toolCalls >= TokenNudgeDefaultMinToolCalls {
		return true, "tool_calls>=min"
	}
	if summaryPath == "" {
		return true, "summary_missing"
	}
	info, err := os.Stat(summaryPath)
	if err == nil {
		ageHours := int(time.Since(info.ModTime()).Hours())
		if ageHours >= TokenNudgeDefaultMaxAgeHours {
			return true, "summary_age>=max"
		}
	}
	return false, "none"
}

type budgetNudgeFlags struct {
	fs          *flag.FlagSet
	root        string
	actor       string
	sessionID   string
	model       string
	transcript  string
	tokensIn    int
	tokensOut   int
	toolCalls   int
	threshold   int
	summaryPath string
}

func newBudgetNudgeFlags(name string) *budgetNudgeFlags {
	f := &budgetNudgeFlags{}
	f.fs = flag.NewFlagSet(name, flag.ContinueOnError)
	f.fs.StringVar(&f.root, "root", "", "Project root")
	f.fs.StringVar(&f.actor, "actor", "agent-sync", "Origem (claude|codex|opencode|cursor|agy|agent-sync)")
	f.fs.StringVar(&f.sessionID, "session-id", "", "ID da sessao")
	f.fs.StringVar(&f.model, "model", "", "Modelo ativo")
	f.fs.StringVar(&f.transcript, "transcript", "", "Caminho do transcript (se aplicavel)")
	f.fs.IntVar(&f.tokensIn, "tokens-in", 0, "Tokens input (0 = extrair do transcript se houver)")
	f.fs.IntVar(&f.tokensOut, "tokens-out", 0, "Tokens output (0 = extrair do transcript se houver)")
	f.fs.IntVar(&f.toolCalls, "tool-calls", 0, "Contador de tool calls para heuristica legada")
	f.fs.IntVar(&f.threshold, "threshold", TokenNudgeDefaultThresholdPct, "Threshold % para nudge")
	f.fs.StringVar(&f.summaryPath, "summary", ".agent-sync/summary.md", "Caminho do summary.md (vazio = desativa heuristica)")
	return f
}

// runBudgetNudge implementa `agent-sync budget nudge`. Constroi o payload
// token-budget-status.json e imprime em stdout.
//
// Se transcript for fornecido e tokensIn/tokensOut = 0, tenta extrair
// tokens do JSONL (mesmo algoritmo de agent-task-record.stop.sh).
func runBudgetNudge(args []string) error {
	f := newBudgetNudgeFlags("agent-sync budget nudge")
	if err := f.fs.Parse(args); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	details := map[string]interface{}{}
	tokensIn, tokensOut := f.tokensIn, f.tokensOut
	if f.transcript != "" && tokensIn == 0 && tokensOut == 0 {
		extIn, extOut, ok := extractTokensFromTranscript(f.transcript)
		if ok {
			tokensIn, tokensOut = extIn, extOut
			details["transcript_path"] = f.transcript
		}
	}
	if f.sessionID == "" {
		// Tentar descobrir pelo SessionState do projeto.
		if s, err := state.ReadSessionState(root); err == nil && s.Session.ID != "" {
			f.sessionID = s.Session.ID
		}
	}
	if f.transcript != "" {
		if _, err := os.Stat(f.transcript); err == nil {
			details["transcript_path"] = f.transcript
		}
	}
	summaryPath := f.summaryPath
	if summaryPath != "" && root != "" && !filepath.IsAbs(summaryPath) {
		summaryPath = filepath.Join(root, summaryPath)
	}
	status, err := BuildTokenBudgetStatus(f.actor, f.sessionID, f.model,
		tokensIn, tokensOut, f.toolCalls, summaryPath, f.threshold, details)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(&status, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// extractTokensFromTranscript extrai tokens_in/out do ultimo evento
// `assistant` em um transcript JSONL (Claude Code / Codex). Retorna
// (0, 0, false) quando transcript ausente, nao-parseable ou sem campo
// usage. Heuristica equivalente a agent-task-record.stop.sh.
func extractTokensFromTranscript(path string) (int, int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	lines := strings.Split(string(data), "\n")
	// Percorre de tras pra frente ate achar assistant.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if !strings.Contains(line, `"type":"assistant"`) {
			continue
		}
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		// Tentar .message.usage.input_tokens OU .usage.input_tokens OU
		// .message.usage.prompt_tokens.
		usage, _ := ev["usage"].(map[string]interface{})
		if usage == nil {
			if msg, ok := ev["message"].(map[string]interface{}); ok {
				usage, _ = msg["usage"].(map[string]interface{})
			}
		}
		if usage == nil {
			return 0, 0, false
		}
		var inT, outT int
		if v, ok := usage["input_tokens"].(float64); ok {
			inT = int(v)
		} else if v, ok := usage["prompt_tokens"].(float64); ok {
			inT = int(v)
		}
		if v, ok := usage["output_tokens"].(float64); ok {
			outT = int(v)
		} else if v, ok := usage["completion_tokens"].(float64); ok {
			outT = int(v)
		}
		return inT, outT, true
	}
	return 0, 0, false
}
