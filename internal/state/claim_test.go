package state

// claim_test.go: testes do A-52 (state claim/release, claim-once handoff).
//
// Cobre 4 cenarios via 2 top-level tests:
//   TestStateClaim: claim novo, claim duplicado, claim de ID inexistente.
//   TestStateRelease: release de claim proprio, release de ID sem claim,
//   release por outro claimer.
//
// Padrao: seed via sampleState() (3 tasks T-1..T-3), write com runStateWrite,
// depois runStateClaim/runStateRelease, read com ReadSessionState + s.normalize.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedSampleState faz write de sampleState() no root temporario. Retorna
// o root (ja com .agent-sync/session-state.json gravado e schema-valido).
func seedSampleState(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, SessionStateDirName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	good := sampleState()
	goodBytes, err := marshalSessionState(good)
	if err != nil {
		t.Fatalf("marshal sampleState: %v", err)
	}
	tmp := filepath.Join(t.TempDir(), "seed.json")
	if err := os.WriteFile(tmp, goodBytes, 0o644); err != nil {
		t.Fatalf("write seed.json: %v", err)
	}
	if err := runStateWrite([]string{"-root", root, "-from", tmp}); err != nil {
		t.Fatalf("runStateWrite seed: %v", err)
	}
}

// TestStateClaim cobre A-52: claim novo, claim duplicado, claim de ID inexistente.
func TestStateClaim(t *testing.T) {
	root := t.TempDir()
	seedSampleState(t, root)

	// Cenario 1: claim novo em T-1 (status=done, mas ID existe).
	if err := runStateClaim([]string{"T-1", "-root", root, "-by", "opencode-A"}); err != nil {
		t.Fatalf("claim novo T-1 falhou: %v", err)
	}
	got := readStateForTest(t, root)
	got.normalize()
	if len(got.Tasks) == 0 || got.Tasks[0].ID != "T-1" {
		t.Fatalf("Tasks[0] esperada T-1, obtido %+v", got.Tasks)
	}
	if got.Tasks[0].ClaimedBy != "opencode-A" {
		t.Errorf("ClaimedBy esperado 'opencode-A', obtido %q", got.Tasks[0].ClaimedBy)
	}
	if got.Tasks[0].ClaimedAt.IsZero() {
		t.Errorf("ClaimedAt esperado nao-zero")
	}

	// Cenario 2: claim duplicado por outro claimer -> erro "ja reivindicada".
	err := runStateClaim([]string{"T-1", "-root", root, "-by", "opencode-B"})
	if err == nil {
		t.Fatalf("claim duplicado deveria falhar")
	}
	if !strings.Contains(err.Error(), "ja reivindicada por") {
		t.Errorf("erro esperado mencionar 'ja reivindicada por', obtido: %v", err)
	}

	// Cenario 3: mesmo claimer pode re-reivindicar (idempotente para o dono).
	if err := runStateClaim([]string{"T-1", "-root", root, "-by", "opencode-A"}); err != nil {
		t.Errorf("re-claim pelo mesmo owner deveria passar: %v", err)
	}

	// Cenario 4: claim de ID inexistente -> erro "nao encontrada".
	err = runStateClaim([]string{"T-9999", "-root", root, "-by", "opencode-A"})
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

// marshalSessionState encapsula json.MarshalIndent para reuso nos testes.
// Existe aqui (nao em outro lugar) porque claim_test.go precisa serializar
// SessionState -> bytes sem depender de json na assinatura.
func marshalSessionState(s SessionState) ([]byte, error) {
	return json.MarshalIndent(&s, "", "  ")
}

// readStateForTest le .agent-sync/session-state.json e desserializa.
// Helper interno (nao exportado) — claim_test.go e o pacote internal/state.
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
