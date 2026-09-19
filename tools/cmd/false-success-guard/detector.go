// Lexical false-success detector — pattern proposed in arXiv 2606.09863v1:
// a lightweight regex classifier flags confident success language without
// attached evidence (path:line, command output, test result) far more
// reliably and 3+ orders of magnitude cheaper than using an LLM as judge
// (the paper reports AUROC 0.83-0.95 for the lexical detector vs 0.64 for
// a GPT-4o judge, which anchors on confident wording instead of outcome).
//
// This is advisory, not a gate: at the paper's usable flag rate, roughly
// half of what it flags is a false positive, so it must never block —
// only nudge the agent to double-check before declaring success.
package main

import "regexp"

// successAssertionPattern mirrors the paper's FS_ASSERT_RE: language that
// asserts a task outcome as already accomplished.
var successAssertionPattern = regexp.MustCompile(`(?i)\b(concluíd[oa]|conclu[ií]do com sucesso|feito com sucesso|pronto|all set|successfully|done|completed|resolvido|corrigido|implementado)\b`)

// honestFailurePattern mirrors HONEST_FAILURE_RE: language that already
// admits failure or a blocker, so it must never be flagged.
var honestFailurePattern = regexp.MustCompile(`(?i)\b(não consegui|não foi possível|falhou|erro ao|unable to|i cannot|i can't|não verifiquei|não foi verificado|bloqueado|blocked)\b`)

// evidencePattern looks for markers that ground a success claim in
// something checkable: a file:line reference, a command/test result (EN or
// PT-BR), a fenced or inline code span citing a real command/tool, or a
// shell prompt. Inline single-backtick spans are included because citing a
// specific command/tool name (`go test`, `gofmt`) is how this project's own
// verification is normally reported — a fenced block is not the only way
// evidence shows up in prose (found live: a real "tudo passou" verification
// citing `go test`/`gofmt` was flagged before this pattern existed).
var evidencePattern = regexp.MustCompile(`(?i)(\S+\.\w+:\d+|exit code|passed|failed|passou|` + "```|`[^`\n]+`" + `|\$ \S+)`)

// ExecutionEvidence captures observable runtime trace step evidence
// inspired by the HarnessFix framework (arXiv 2606.06324v2).
// When available from the transcript, it grounds success assertions
// in verified state changes or successful verification tool execution.
type ExecutionEvidence struct {
	HasTraceData   bool
	HasMutation    bool // Tool calls like write/edit/patch/file creation
	RanTestCommand bool // Tool calls executing tests or verification commands
	HasToolError   bool // Unhandled errors, non-zero exit codes or [TOOL_STATUS: FAILED]
}

// Verdict is the outcome of classifying one piece of agent-facing text.
type Verdict struct {
	Flagged bool
	Reason  string
}

// Classify applies lexical rules to text (typically the agent's final message)
// and returns whether it looks like an unverified success claim.
func Classify(text string) Verdict {
	return ClassifyWithTrace(text, ExecutionEvidence{HasTraceData: false})
}

// ClassifyWithTrace combines lexical detection with runtime trace step evidence.
func ClassifyWithTrace(text string, ev ExecutionEvidence) Verdict {
	if honestFailurePattern.MatchString(text) {
		return Verdict{Flagged: false, Reason: "linguagem de falha honesta detectada"}
	}
	if !successAssertionPattern.MatchString(text) {
		return Verdict{Flagged: false, Reason: "nenhuma alegação de sucesso detectada"}
	}

	// Trace-grounded evaluation (HarnessFix)
	if ev.HasTraceData {
		// If tool errors were detected and unhandled
		if ev.HasToolError {
			return Verdict{
				Flagged: true,
				Reason:  "alegação de sucesso mas há erro não tratado ou falha de ferramenta no turno recente",
			}
		}

		// If the assertion claims success/completion, but there was neither mutation nor verification
		if !ev.HasMutation && !ev.RanTestCommand {
			return Verdict{
				Flagged: true,
				Reason:  "alegação de sucesso sem transição de estado (mutação) ou comando de validação no turno recente",
			}
		}

		// Verified by runtime evidence
		return Verdict{
			Flagged: false,
			Reason:  "alegação de sucesso confirmada por evidência de execução no trace",
		}
	}

	// Fallback to purely lexical rules if no trace data was available
	if evidencePattern.MatchString(text) {
		return Verdict{Flagged: false, Reason: "alegação de sucesso com evidência anexada"}
	}
	return Verdict{
		Flagged: true,
		Reason:  "alegação de sucesso sem evidência anexada (path:line, saída de comando/teste)",
	}
}

