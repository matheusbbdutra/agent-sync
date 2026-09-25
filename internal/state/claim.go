// claim.go: subcommands CLI para 'agent-sync state claim <A-N>' e
// 'state release <A-N>' — claim-once handoff (A-52).
//
// Origem: ai-memory memory_handoff_begin/list/accept/cancel (akitaonrails).
// Lockless (D-2): campos claimed_by (string CLI kind) + claimed_at (RFC3339)
// gravados direto em .agent-sync/session-state.json. Sem cross-process lockfile.
// Backward compatible: tasks legadas tem claimed_by='' (read ignora reivindicacoes
// ha mais de AGENT_SYNC_HANDOFF_TIMEOUT, default 1h).
//
// Cliente identifica o dono via flag -by <cli-kind> (default = host do binario).
package state

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// defaultHandoffTimeout e o periodo maximo que um claim fica valido.
// Quando claimed_at < now - timeout, o claim e considerado expirado e
// outro cliente pode reivindicar a mesma tarefa. Configuravel via
// AGENT_SYNC_HANDOFF_TIMEOUT (Go duration string: "1h", "30m", etc).
const defaultHandoffTimeout = time.Hour

// runStateClaim implementa `agent-sync state claim <A-N>` (subcommand A-52).
func runStateClaim(args []string) error {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("claim: ID da tarefa obrigatorio (uso: state claim <A-N>)")
	}
	id := strings.TrimSpace(args[0])
	f := newStateFlags("agent-sync state claim")
	f.fs.String("by", "", "Quem esta reivindicando (default: hostname do binario)")
	if err := f.parse(args[1:], os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	claimer := strings.TrimSpace(f.fs.Lookup("by").Value.String())
	if claimer == "" {
		claimer, _ = os.Hostname()
	}
	timeout, err := handoffTimeout()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(statePath(root))
	if err != nil {
		return err
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		return fmt.Errorf("claim: schema invalido: %w", err)
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	s.normalize()
	if err := claimTask(&s, id, claimer, timeout); err != nil {
		return err
	}
	return WriteSessionState(root, s)
}

// runStateRelease implementa `agent-sync state release <A-N>`.
func runStateRelease(args []string) error {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		return errors.New("release: ID da tarefa obrigatorio (uso: state release <A-N>)")
	}
	id := strings.TrimSpace(args[0])
	f := newStateFlags("agent-sync state release")
	f.fs.String("by", "", "Quem esta liberando (default: hostname do binario)")
	if err := f.parse(args[1:], os.Stderr); err != nil {
		return err
	}
	root, err := pathutil.ResolveStateRoot(f.root)
	if err != nil {
		return err
	}
	claimer := strings.TrimSpace(f.fs.Lookup("by").Value.String())
	if claimer == "" {
		claimer, _ = os.Hostname()
	}
	data, err := os.ReadFile(statePath(root))
	if err != nil {
		return err
	}
	if err := jsonschemaValidateRaw(data); err != nil {
		return fmt.Errorf("release: schema invalido: %w", err)
	}
	s, err := ReadSessionState(root)
	if err != nil {
		return err
	}
	s.normalize()
	if err := releaseTask(&s, id, claimer); err != nil {
		return err
	}
	return WriteSessionState(root, s)
}

// claimTask aplica o claim-once a uma tarefa identificada por id.
// Erros:
//   - "tarefa nao encontrada"
//   - "ja reivindicado por X ha Ys" (claim ativo dentro do timeout)
//   - nil (sucesso)
func claimTask(s *SessionState, id, claimer string, timeout time.Duration) error {
	now := time.Now().UTC()
	for i := range s.Tasks {
		t := &s.Tasks[i]
		if t.ID != id {
			continue
		}
		if t.ClaimedBy != "" && !isExpired(t.ClaimedAt, now, timeout) && t.ClaimedBy != claimer {
			age := now.Sub(t.ClaimedAt).Round(time.Second)
			return fmt.Errorf("tarefa %q ja reivindicada por %q ha %s (timeout %s)",
				t.ID, t.ClaimedBy, age, timeout)
		}
		t.ClaimedBy = claimer
		t.ClaimedAt = now
		return nil
	}
	return fmt.Errorf("tarefa %q nao encontrada em .agent-sync/session-state.json", id)
}

// releaseTask libera o claim. Falha se a tarefa nao existe ou se foi reivindicada
// por outro cliente (sem timeout: lock-by-identity).
func releaseTask(s *SessionState, id, claimer string) error {
	for i := range s.Tasks {
		t := &s.Tasks[i]
		if t.ID != id {
			continue
		}
		if t.ClaimedBy == "" {
			return fmt.Errorf("tarefa %q nao tem claim ativo", t.ID)
		}
		if t.ClaimedBy != claimer {
			return fmt.Errorf("tarefa %q reivindicada por %q, nao por %q",
				t.ID, t.ClaimedBy, claimer)
		}
		t.ClaimedBy = ""
		t.ClaimedAt = time.Time{}
		return nil
	}
	return fmt.Errorf("tarefa %q nao encontrada em .agent-sync/session-state.json", id)
}

func isExpired(claimedAt time.Time, now time.Time, timeout time.Duration) bool {
	if claimedAt.IsZero() {
		return true
	}
	return now.Sub(claimedAt) > timeout
}

// handoffTimeout le AGENT_SYNC_HANDOFF_TIMEOUT (Go duration) ou retorna default.
func handoffTimeout() (time.Duration, error) {
	v := os.Getenv("AGENT_SYNC_HANDOFF_TIMEOUT")
	if v == "" {
		return defaultHandoffTimeout, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("AGENT_SYNC_HANDOFF_TIMEOUT invalido: %w", err)
	}
	return d, nil
}
