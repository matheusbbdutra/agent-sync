package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// claim_test.go: testes do A-52 (state claim/release, claim-once handoff).
//
// Combina 2 estrategias:
//   - Unit tests (claimTask/releaseTask/handTimeout): rapidos, cobrem edge cases
//     (timeout expirado, mesmo claimer refresh, claim bloqueado, etc).
//   - Integration tests (runStateClaim/runStateRelease + runStateWrite): lentos
//     mas cobrem o roundtrip CLI -> disco -> JSON -> CLI, garantindo que wiramento
//     wirado em render.go:60-62 funciona end-to-end com schema embedded.
//
// Origem: 11 testes unitarios introduzidos em D-95 + 2 integration tests do
// first commit (938e683). Merge preserva ambos coverage (unit granular +
// integration end-to-end).

// ---------- Integration tests ----------

// seedSampleState grava sampleState() direto em .agent-sync/session-state.json
// do root temporario. Bypassa runStateWrite (que nao aceita reader) escrevendo
// o arquivo via os.WriteFile apos validar com schema embedded.
func seedSampleState(t *testing.T, root string) {
	t.Helper()
	data, err := marshalSessionState(sampleState())
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	dir := filepath.Join(root, SessionStateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .agent-sync: %v", err)
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		t.Fatalf("schema validate seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SessionStateFileName), data, 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
}

// TestStateClaim cobre A-52: claim novo, claim duplicado, claim de ID inexistente.
func TestStateClaim(t *testing.T) {
	root := t.TempDir()
	seedSampleState(t, root)

	// Cenario 1: claim novo de T-1 -> ok e claimed_by='claude-code'.
	if err := runStateClaim([]string{"T-1", "-root", root, "-by", "claude-code"}); err != nil {
		t.Fatalf("claim T-1 falhou: %v", err)
	}
	got := readStateForTest(t, root)
	got.normalize()
	for _, task := range got.Tasks {
		if task.ID == "T-1" {
			if task.ClaimedBy != "claude-code" {
				t.Errorf("ClaimedBy esperado 'claude-code', obtido %q", task.ClaimedBy)
			}
			if task.ClaimedAt.IsZero() {
				t.Errorf("ClaimedAt nao setado")
			}
		}
	}

	// Cenario 2: claim duplicado por outro claimer -> erro.
	err := runStateClaim([]string{"T-1", "-root", root, "-by", "codex"})
	if err == nil {
		t.Fatalf("claim duplicado deveria falhar")
	}
	if !strings.Contains(err.Error(), "ja reivindicada por") {
		t.Errorf("erro esperado mencionar 'ja reivindicada por', obtido: %v", err)
	}

	// Cenario 3: claim de ID inexistente -> erro.
	err = runStateClaim([]string{"T-9999", "-root", root, "-by", "claude-code"})
	if err == nil {
		t.Fatalf("claim de ID inexistente deveria falhar")
	}
	if !strings.Contains(err.Error(), "nao encontrada") {
		t.Errorf("erro esperado mencionar 'nao encontrada', obtido: %v", err)
	}
}

// TestStateRelease cobre A-52: release proprio, release por outro, release sem claim,
// release de ID inexistente.
func TestStateRelease(t *testing.T) {
	root := t.TempDir()
	seedSampleState(t, root)

	// Setup: T-2 vai ser reivindicada por opencode-A.
	if err := runStateClaim([]string{"T-2", "-root", root, "-by", "opencode-A"}); err != nil {
		t.Fatalf("setup claim T-2: %v", err)
	}

	// Cenario 1: release pelo proprio claimer -> ok e claimed_by=''.
	if err := runStateRelease([]string{"T-2", "-root", root, "-by", "opencode-A"}); err != nil {
		t.Fatalf("release proprio T-2 falhou: %v", err)
	}
	got := readStateForTest(t, root)
	got.normalize()
	for _, task := range got.Tasks {
		if task.ID == "T-2" {
			if task.ClaimedBy != "" {
				t.Errorf("ClaimedBy esperado '' apos release, obtido %q", task.ClaimedBy)
			}
			if !task.ClaimedAt.IsZero() {
				t.Errorf("ClaimedAt esperado zero apos release, obtido %v", task.ClaimedAt)
			}
		}
	}

	// Cenario 2: release de task sem claim -> erro "nao tem claim ativo".
	err := runStateRelease([]string{"T-2", "-root", root, "-by", "opencode-A"})
	if err == nil {
		t.Fatalf("release sem claim deveria falhar")
	}
	if !strings.Contains(err.Error(), "nao tem claim ativo") {
		t.Errorf("erro esperado mencionar 'nao tem claim ativo', obtido: %v", err)
	}

	// Cenario 3: re-claim por B e tentar release por A -> erro "reivindicada por B".
	if err := runStateClaim([]string{"T-2", "-root", root, "-by", "opencode-B"}); err != nil {
		t.Fatalf("setup claim T-2 por B: %v", err)
	}
	err = runStateRelease([]string{"T-2", "-root", root, "-by", "opencode-A"})
	if err == nil {
		t.Fatalf("release por outro claimer deveria falhar")
	}
	if !strings.Contains(err.Error(), "reivindicada por") {
		t.Errorf("erro esperado mencionar 'reivindicada por', obtido: %v", err)
	}

	// Cenario 4: release de ID inexistente -> erro.
	err = runStateRelease([]string{"T-9999", "-root", root, "-by", "opencode-B"})
	if err == nil {
		t.Fatalf("release de ID inexistente deveria falhar")
	}
	if !strings.Contains(err.Error(), "nao encontrada") {
		t.Errorf("erro esperado mencionar 'nao encontrada', obtido: %v", err)
	}
}

// ---------- Unit tests (claimTask / releaseTask / handoffTimeout) ----------

func TestClaimTaskSuccess(t *testing.T) {
	s := fixtureStateWithTask("A-52", "claim-once handoff")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.Tasks[0].ClaimedBy != "claude-code" {
		t.Fatalf("ClaimedBy nao setado: %q", s.Tasks[0].ClaimedBy)
	}
	if s.Tasks[0].ClaimedAt.IsZero() {
		t.Fatal("ClaimedAt nao setado")
	}
}

func TestClaimTaskNotFound(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	err := claimTask(&s, "A-99", "claude-code", time.Hour)
	if err == nil {
		t.Fatal("esperava erro para task inexistente")
	}
}

func TestClaimTaskBlockedByOtherWithinTimeout(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	err := claimTask(&s, "A-52", "codex", time.Hour)
	if err == nil {
		t.Fatal("esperava erro: claude ja reivindicou")
	}
}

func TestClaimTaskSameClaimerRefreshesTimestamp(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	first := s.Tasks[0].ClaimedAt
	time.Sleep(10 * time.Millisecond)
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatalf("re-claim pelo mesmo claimer deveria passar: %v", err)
	}
	if !s.Tasks[0].ClaimedAt.After(first) {
		t.Fatal("re-claim deveria atualizar timestamp")
	}
}

func TestClaimTaskExpiredAllowsNewClaimer(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	// simular claim expirado: claimed_at = 2h atras, timeout 1h
	s.Tasks[0].ClaimedAt = time.Now().UTC().Add(-2 * time.Hour)
	if err := claimTask(&s, "A-52", "codex", time.Hour); err != nil {
		t.Fatalf("claim expirado deveria ser re-claimable: %v", err)
	}
	if s.Tasks[0].ClaimedBy != "codex" {
		t.Fatalf("ClaimedBy nao transferido: %q", s.Tasks[0].ClaimedBy)
	}
}

func TestReleaseTaskSuccess(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := releaseTask(&s, "A-52", "claude-code"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if s.Tasks[0].ClaimedBy != "" || !s.Tasks[0].ClaimedAt.IsZero() {
		t.Fatalf("release nao limpou: %+v", s.Tasks[0])
	}
}

func TestReleaseTaskByWrongClaimer(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	if err := claimTask(&s, "A-52", "claude-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	err := releaseTask(&s, "A-52", "codex")
	if err == nil {
		t.Fatal("esperava erro: codex tentando liberar claim de claude")
	}
}

func TestReleaseTaskNoClaim(t *testing.T) {
	s := fixtureStateWithTask("A-52", "x")
	err := releaseTask(&s, "A-52", "claude-code")
	if err == nil {
		t.Fatal("esperava erro: sem claim ativo")
	}
}

func TestHandoffTimeoutDefault(t *testing.T) {
	os.Unsetenv("AGENT_SYNC_HANDOFF_TIMEOUT")
	d, err := handoffTimeout()
	if err != nil {
		t.Fatal(err)
	}
	if d != time.Hour {
		t.Fatalf("default esperado 1h, obtive %s", d)
	}
}

func TestHandoffTimeoutEnv(t *testing.T) {
	os.Setenv("AGENT_SYNC_HANDOFF_TIMEOUT", "30m")
	defer os.Unsetenv("AGENT_SYNC_HANDOFF_TIMEOUT")
	d, err := handoffTimeout()
	if err != nil {
		t.Fatal(err)
	}
	if d != 30*time.Minute {
		t.Fatalf("esperava 30m, obtive %s", d)
	}
}

func TestHandoffTimeoutInvalid(t *testing.T) {
	os.Setenv("AGENT_SYNC_HANDOFF_TIMEOUT", "invalid")
	defer os.Unsetenv("AGENT_SYNC_HANDOFF_TIMEOUT")
	_, err := handoffTimeout()
	if err == nil {
		t.Fatal("esperava erro para duracao invalida")
	}
}

// ---------- Helpers ----------

// fixtureStateWithTask cria SessionState com 1 task livre.
func fixtureStateWithTask(id, title string) SessionState {
	return SessionState{
		Tasks: []SessionTask{{
			ID: id, Title: title, Status: "pending",
		}},
	}
}

// marshalSessionState encapsula json.MarshalIndent para reuso nos testes.
func marshalSessionState(s SessionState) ([]byte, error) {
	return json.MarshalIndent(&s, "", "  ")
}

// readStateForTest le .agent-sync/session-state.json e desserializa.
func readStateForTest(t *testing.T, root string) SessionState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, SessionStateDirName, SessionStateFileName))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		t.Fatalf("schema validate: %v", err)
	}
	s, err := ReadSessionState(root)
	if err != nil {
		t.Fatalf("ReadSessionState: %v", err)
	}
	return s
}