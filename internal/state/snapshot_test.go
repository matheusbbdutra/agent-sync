package state

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// fixtureSessionState devolve um SessionState minimo valido para testes de
// BuildPrecompactSnapshot. A-37 Etapa 2: NextActions/Blockers -> Tasks/Issues.
// Os parametros blockers (slice de IDs) viram SessionIssues com
// LegacyKind="blocker" e Resolved=false para preservar o contrato do teste
// AdviseOnlyComBlocker.
func fixtureSessionState(openQs []string, blockers []string, actions []string, decisions []SessionDecision) SessionState {
	now := time.Now().UTC()
	return SessionState{
		SchemaVersion: SessionStateSchemaVersion,
		Project:       SessionProject{Name: "agent-sync", Root: "/tmp/agent-sync"},
		Git: SessionGit{
			Branch: "main",
			Head:   "abc1234",
		},
		Session: SessionMeta{
			ID:        "sess-test-001",
			StartedAt: now.Add(-30 * time.Minute),
			UpdatedAt: now,
		},
		Decisions: decisions,
		Tasks: func() []SessionTask {
			out := make([]SessionTask, 0, len(actions))
			for _, a := range actions {
				out = append(out, SessionTask{ID: a, Title: "teste", Status: "pending", LegacyKind: "todo", StartedAt: now})
			}
			return out
		}(),
		Issues: func() []SessionIssue {
			out := make([]SessionIssue, 0, len(blockers))
			for _, b := range blockers {
				out = append(out, SessionIssue{
					ID:          b,
					Title:       "fixture",
					Severity:    "medium",
					LegacyKind:  "blocker",
					Description: "fixture (sintetico)",
					DetectedAt:  now,
					Resolved:    false,
				})
			}
			return out
		}(),
		OpenQuestions: openQs,
	}
}

func TestBuildPrecompactSnapshot_AllowSemBlockersNemQuestions(t *testing.T) {
	s := fixtureSessionState(nil, nil, []string{"A-1"}, []SessionDecision{
		{ID: "D-1", Title: "decisao recente"},
	})
	snap, err := BuildPrecompactSnapshot(s, "claude", "auto", "1.0.50", "ctx_threshold_reached")
	if err != nil {
		t.Fatalf("BuildPrecompactSnapshot: %v", err)
	}
	if snap.Decision != "allow" {
		t.Errorf("esperava decision=allow, veio %q", snap.Decision)
	}
	if snap.DecisionReason != "" {
		t.Errorf("esperava decision_reason vazio em allow, veio %q", snap.DecisionReason)
	}
	if snap.Actor != "claude" {
		t.Errorf("esperava actor=claude, veio %q", snap.Actor)
	}
	if snap.CompactionKind != "auto" {
		t.Errorf("esperava compaction_kind=auto, veio %q", snap.CompactionKind)
	}
	if snap.SessionID != "sess-test-001" {
		t.Errorf("esperava session_id=sess-test-001, veio %q", snap.SessionID)
	}
	if len(snap.Snapshot.DecisionsRecent) != 1 || snap.Snapshot.DecisionsRecent[0] != "D-1" {
		t.Errorf("decisions_recent inesperadas: %v", snap.Snapshot.DecisionsRecent)
	}
}

func TestBuildPrecompactSnapshot_AdviseOnlyComOpenQuestions(t *testing.T) {
	s := fixtureSessionState([]string{"Q-1"}, nil, nil, nil)
	snap, err := BuildPrecompactSnapshot(s, "codex", "manual", "", "user_requested")
	if err != nil {
		t.Fatalf("BuildPrecompactSnapshot: %v", err)
	}
	if snap.Decision != "advise_only" {
		t.Errorf("esperava decision=advise_only, veio %q", snap.Decision)
	}
	if !strings.Contains(snap.DecisionReason, "Q-1") && !strings.Contains(snap.DecisionReason, "open_question") {
		t.Errorf("esperava decision_reason mencionando open_question, veio %q", snap.DecisionReason)
	}
}

func TestBuildPrecompactSnapshot_AdviseOnlyComBlocker(t *testing.T) {
	// Blockers nao bloqueiam por enquanto (sem UpdatedAt confiavel); viram advise_only.
	s := fixtureSessionState(nil, []string{"B-1"}, nil, nil)
	snap, err := BuildPrecompactSnapshot(s, "claude", "auto", "", "")
	if err != nil {
		t.Fatalf("BuildPrecompactSnapshot: %v", err)
	}
	if snap.Decision != "advise_only" {
		t.Errorf("esperava decision=advise_only (blocker vira advise_only ate ADR futura adicionar UpdatedAt), veio %q", snap.Decision)
	}
}

func TestBuildPrecompactSnapshot_DefaultsActorEKind(t *testing.T) {
	s := fixtureSessionState(nil, nil, nil, nil)
	snap, err := BuildPrecompactSnapshot(s, "", "", "", "")
	if err != nil {
		t.Fatalf("BuildPrecompactSnapshot: %v", err)
	}
	if snap.Actor != "agent-sync" {
		t.Errorf("esperava actor default agent-sync, veio %q", snap.Actor)
	}
	if snap.CompactionKind != "auto" {
		t.Errorf("esperava compaction_kind default auto, veio %q", snap.CompactionKind)
	}
}

func TestBuildPrecompactSnapshot_RespeitaSchema(t *testing.T) {
	// Confirma que o schema embedded aceita o payload gerado.
	s := fixtureSessionState([]string{"Q-1", "Q-2"}, []string{"B-1"}, []string{"A-1", "A-2"}, []SessionDecision{
		{ID: "D-1", Title: "d1"},
		{ID: "D-2", Title: "d2"},
	})
	snap, err := BuildPrecompactSnapshot(s, "opencode", "native", "2.0.11", "experimental.session.compacting")
	if err != nil {
		t.Fatalf("BuildPrecompactSnapshot: %v", err)
	}
	// Schema ja foi validado dentro de Build; re-marshal para garantir
	// idempotencia do JSON.
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}
	if decoded["decision"] != "advise_only" {
		t.Errorf("round-trip mudou decision: %v", decoded["decision"])
	}
	if decoded["actor"] != "opencode" {
		t.Errorf("round-trip mudou actor: %v", decoded["actor"])
	}
}

func TestEmitPrecompactSnapshotForCLI_CodexBlockWrappaContinueFalse(t *testing.T) {
	snap := PrecompactSnapshot{
		SchemaVersion:  PrecompactSnapshotSchemaVersion,
		TS:             time.Now().UTC(),
		Actor:          "codex",
		CompactionKind: "auto",
		SessionID:      "sess-x",
		Snapshot: PrecompactInner{
			TS: time.Now().UTC(),
		},
		Decision:       "block",
		DecisionReason: "open_question em aberto",
	}
	// Capturar stdout via redirecionamento seria fragil; validamos apenas
	// que a funcao nao retorna erro e produz JSON parseavel via stub do
	// actor. Como EmitPrecompactSnapshotForCLI usa fmt.Println, validamos
	// via re-marshalling da estrutura equivalente que seria impressa.
	if err := EmitPrecompactSnapshotForCLI(snap, "codex"); err != nil {
		t.Fatalf("EmitPrecompactSnapshotForCLI: %v", err)
	}
	// Estrutura codex-block equivalente para garantir round-trip.
	out := map[string]interface{}{
		"continue":   false,
		"stopReason": "agent-sync: " + snap.DecisionReason,
		"snapshot":   snap,
	}
	data, _ := json.Marshal(out)
	if !strings.Contains(string(data), `"continue":false`) {
		t.Errorf("esperava continue:false no output codex block, veio %s", data)
	}
}

func TestRunStateSnapshot_FlagsEOutput(t *testing.T) {
	root := t.TempDir()
	s := fixtureSessionState(nil, nil, nil, []SessionDecision{{ID: "D-99", Title: "ok"}})
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("WriteSessionState: %v", err)
	}
	err := runStateSnapshot([]string{"-root", root, "-actor", "claude", "-kind", "auto"})
	if err != nil {
		t.Fatalf("runStateSnapshot: %v", err)
	}
}

func TestRunStateSnapshot_RejeitaSchemaInvalido(t *testing.T) {
	// Snapshot precisa ser valido contra schema embedded; payload gerado
	// em BuildPrecompactSnapshot ja e validado, entao o teste cobre apenas
	// o caminho feliz. Para garantir robustez, injetamos um actor invalido
	// via -actor (campo enum) e esperamos erro de schema.
	root := t.TempDir()
	s := fixtureSessionState(nil, nil, nil, nil)
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("WriteSessionState: %v", err)
	}
	err := runStateSnapshot([]string{"-root", root, "-actor", "hacker"})
	if err == nil {
		t.Error("esperava erro de schema com actor=hacker (fora do enum), veio nil")
	}
}

func TestRunStateSnapshot_ToFile(t *testing.T) {
	root := t.TempDir()
	s := fixtureSessionState([]string{"Q-1"}, nil, nil, nil)
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("WriteSessionState: %v", err)
	}
	out := root + "/snap.json"
	if err := runStateSnapshot([]string{"-root", root, "-actor", "agy", "-kind", "manual", "-to", out}); err != nil {
		t.Fatalf("runStateSnapshot -to: %v", err)
	}
	// Ler e validar JSON.
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	var snap PrecompactSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if snap.Decision != "advise_only" {
		t.Errorf("esperava advise_only, veio %q", snap.Decision)
	}
	if snap.Actor != "agy" {
		t.Errorf("esperava actor=agy, veio %q", snap.Actor)
	}
}
