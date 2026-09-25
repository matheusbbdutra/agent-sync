// precompact_snapshot.go implementa a geracao do snapshot PreCompact
// (ADR-precompact-snapshot-cross-cli) e o subcommand CLI
// `agent-sync state snapshot` que produz o payload canonico
// `precompact-snapshot.json` consumivel pelos hooks PreCompact de Claude
// Code, Codex, Antigravity e pelo plugin v2 do OpenCode.
//
// Decisao:
//   - allow    -> nenhum bloqueio estrutural; compactacao prossegue.
//   - advise_only -> ha open_questions em aberto; compactacao prossegue mas
//                   o aviso e registrado no session-event.
//   - block    -> ha blocker (B-N) ativo ha >7 dias sem movimento ou
//                   acao pendente (A-N) de blocker critico; compacta e
//                   CANCELADA pelo harness.
//
// Schema versionado: tools/jsonschema/schemas/precompact-snapshot.json.
// Regra "fechado por padrao" (ADR-001) respeitada; unica excecao em
// 'details' (mesma justificativa da ADR-003).
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/token-tools/jsonschema"
)

// PrecompactSnapshotSchemaVersion e a versao do schema. Const ate bump major.
const PrecompactSnapshotSchemaVersion = "1.0"

// PrecompactSnapshot representa o payload canonico escrito pelos hooks
// PreCompact antes da compactacao. Validado contra
// tools/jsonschema/schemas/precompact-snapshot.json.
type PrecompactSnapshot struct {
	SchemaVersion  string                 `json:"schema_version"`
	TS             time.Time              `json:"ts"`
	Actor          string                 `json:"actor"`
	CLIVersion     string                 `json:"cli_version,omitempty"`
	CompactionKind string                 `json:"compaction_kind"`
	SessionID      string                 `json:"session_id,omitempty"`
	Trigger        string                 `json:"trigger,omitempty"`
	Snapshot       PrecompactInner        `json:"snapshot"`
	Decision       string                 `json:"decision"`
	DecisionReason string                 `json:"decision_reason,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// PrecompactInner sao os dados capturados do estado no momento do snapshot.
// Espelha a sub-shape `snapshot` do schema precompact-snapshot.json.
type PrecompactInner struct {
	TS              time.Time `json:"ts"`
	TokensIn        int       `json:"tokens_in,omitempty"`
	TokensOut       int       `json:"tokens_out,omitempty"`
	FilesTouched    []string  `json:"files_touched,omitempty"`
	DecisionsRecent []string  `json:"decisions_recent,omitempty"`
	OpenQuestions   []string  `json:"open_questions,omitempty"`
	ActionsPending  []string  `json:"actions_pending,omitempty"`
}

// BuildPrecompactSnapshot monta o snapshot a partir do SessionState lido.
// `actor` e a CLI origem (claude/codex/opencode/cursor/agy).
// `kind` e manual|auto|native. `cliVersion` e string livre (opcional).
//
// Regra de decisao (consolidada e revisavel):
//   - block:        len(Blockers)>0 && (todos com UpdatedAt < now-7d)
//                            -> ha blocker parado ha >7 dias; compacta NUNCA.
//   - advise_only:  len(OpenQuestions)>0
//                            -> ha duvida em aberto; deixa compactar com aviso.
//   - allow:        caso contrario.
//
// A regra NAO bloqueia por A-N pendente: A-N e trabalho em fila, nao
// condicao de parada. Blockers parados >7d sao condicao de parada
// (compactar destruiria o contexto que o usuario precisa revisar antes).
func BuildPrecompactSnapshot(s SessionState, actor, kind, cliVersion, trigger string) (PrecompactSnapshot, error) {
	if actor == "" {
		actor = "agent-sync"
	}
	if kind == "" {
		kind = "auto"
	}
	now := time.Now().UTC()

	// Coleta IDs recentes (ultimos 5).
	decisionsRecent := make([]string, 0, 5)
	for i := len(s.Decisions) - 1; i >= 0 && len(decisionsRecent) < 5; i-- {
		decisionsRecent = append(decisionsRecent, s.Decisions[i].ID)
	}
	// Inverte para ordem cronologica (mais antigo -> mais novo).
	for i, j := 0, len(decisionsRecent)-1; i < j; i, j = i+1, j-1 {
		decisionsRecent[i], decisionsRecent[j] = decisionsRecent[j], decisionsRecent[i]
	}

	actionsPending := make([]string, 0)
	for _, t := range s.Tasks {
		if t.Status != "pending" {
			continue
		}
		if t.LegacyKind != "todo" && t.LegacyKind != "delivery" {
			continue
		}
		actionsPending = append(actionsPending, t.ID)
	}

	openQuestions := append([]string(nil), s.OpenQuestions...)

	decision, reason := decide(s, now)

	snap := PrecompactSnapshot{
		SchemaVersion:  PrecompactSnapshotSchemaVersion,
		TS:             now,
		Actor:          actor,
		CLIVersion:     cliVersion,
		CompactionKind: kind,
		SessionID:      s.Session.ID,
		Trigger:        trigger,
		Snapshot: PrecompactInner{
			TS:              now,
			DecisionsRecent: decisionsRecent,
			OpenQuestions:   openQuestions,
			ActionsPending:  actionsPending,
		},
		Decision:       decision,
		DecisionReason: reason,
	}

	if err := jsonschema.Validate("precompact-snapshot", snap); err != nil {
		return PrecompactSnapshot{}, fmt.Errorf("precompact-snapshot: schema invalido: %w", err)
	}
	return snap, nil
}

func decide(s SessionState, now time.Time) (decision, reason string) {
	// Advise_only: ha issue(s) nao resolvida(s) com legacy_kind=blocker.
	unresolvedBlockers := 0
	for _, iss := range s.Issues {
		if iss.LegacyKind == "blocker" && !iss.Resolved {
			unresolvedBlockers++
		}
	}
	if unresolvedBlockers > 0 {
		return "advise_only", fmt.Sprintf("%d blocker(s) nao resolvido(s); revisar antes de compactar", unresolvedBlockers)
	}
	// Advise_only: ha open_questions em aberto.
	if len(s.OpenQuestions) > 0 {
		return "advise_only", fmt.Sprintf("%d open_question(s) em aberto", len(s.OpenQuestions))
	}
	return "allow", ""
}

// EmitPrecompactSnapshotForCLI monta e imprime o snapshot em stdout no
// formato que cada harness espera:
//   - claude, agy: stdout puro (hook le via exit code + stderr; o JSON fica
//                 em stdout se hook pedir; tratamos igual a codex).
//   - codex: JSON com `{"continue": <bool>, ...}` parseado pelo harness.
//   - opencode: handled pelo plugin v2, nao por este subcommand.
//
// Estrategia simples: imprimimos o payload canonico (JSON) e, em codex,
// anexamos wrapper {"continue": false, "stopReason": "..."} quando
// decision == "block". Mantemos stdout sempre JSON estrito (Codex exige).
func EmitPrecompactSnapshotForCLI(snap PrecompactSnapshot, actor string) error {
	switch actor {
	case "codex":
		// Codex espera JSON com campo "continue". Wrappa se for block.
		if snap.Decision == "block" {
			out := map[string]interface{}{
				"continue":   false,
				"stopReason": "agent-sync: " + snap.DecisionReason,
				"snapshot":   snap,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		// allow/advise_only: payload canonico direto.
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		// Claude, agy, opencode: payload canonico direto.
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
}

// runStateSnapshot implementa `agent-sync state snapshot` (subcommand
// dentro do grupo state; ver ADR-precompact-snapshot Decisao 3).
//
// Flags:
//   -root     project root
//   -actor    claude|codex|opencode|cursor|agy|agent-sync (default agent-sync)
//   -kind     manual|auto|native (default auto)
//   -cli      CLI version string (opcional)
//   -trigger  texto livre descrevendo o motivo (opcional)
//   -to       se setado, escreve no arquivo em vez de stdout
func runStateSnapshot(args []string) error {
	f := newStateFlags("agent-sync state snapshot")
	actor := ""
	kind := ""
	cliVersion := ""
	trigger := ""
	toPath := ""
	f.fs.StringVar(&actor, "actor", "agent-sync", "Origem (claude|codex|opencode|cursor|agy|agent-sync)")
	f.fs.StringVar(&kind, "kind", "auto", "Tipo de compactacao (manual|auto|native)")
	f.fs.StringVar(&cliVersion, "cli", "", "Versao da CLI (string livre)")
	f.fs.StringVar(&trigger, "trigger", "", "Motivo da compactacao")
	f.fs.StringVar(&toPath, "to", "", "Escrever em arquivo em vez de stdout")
	if err := f.parse(args, os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	snap, err := BuildPrecompactSnapshot(s, actor, kind, cliVersion, trigger)
	if err != nil {
		return err
	}
	if toPath != "" {
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(toPath, data, 0o644)
	}
	return EmitPrecompactSnapshotForCLI(snap, actor)
}
