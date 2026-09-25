package state

// state_model_events_test.go: testes de eventos do state model.
// Migrado de session_state_events_test.go em 2026-09-21 (Fase 3).
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/event"
)

func TestWriteSessionStateEmiteStateRenderNaPrimeiraEscrita(t *testing.T) {
	root := setupStateFixture(t, sampleState())
	events, err := event.ReadEvents(root, event.EventReadOptions{})
	if err != nil {
		t.Fatalf("event.ReadEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("esperava >=1 evento (state_render), veio 0")
	}
	if events[0].Kind != "state_render" {
		t.Errorf("primeiro evento deveria ser state_render, veio %q", events[0].Kind)
	}
	if events[0].Actor != "agent-sync" {
		t.Errorf("actor=%q, esperava agent-sync", events[0].Actor)
	}
	if events[0].SessionID != "sess-2026-09-19" {
		t.Errorf("session_id=%q, esperava sess-2026-09-19", events[0].SessionID)
	}
}

// TestWriteSessionStateEmiteEventosParaItemsNovos valida que appendStateDiffEvents
// emite eventos para items novos ou alterados. A-37 Etapa 2 removeu os loops de
// task/action/blocker; agora so emite 'decision' e 'open_question' alem de
// 'state_render'. Teste adapta-se: segunda escrita adiciona 2 Decisions + 1
// OpenQuestion nova (eram 1+1+1 antes).
func TestWriteSessionStateEmiteEventosParaItemsNovos(t *testing.T) {
	root := t.TempDir()
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}

	first := sampleState()
	if err := WriteSessionState(root, first); err != nil {
		t.Fatalf("primeiro write: %v", err)
	}

	second := first
	second.Decisions = append(second.Decisions,
		SessionDecision{
			ID:        "D-99",
			Title:     "decisao nova 99",
			Rationale: "teste",
			MadeAt:    first.Session.StartedAt,
		},
		SessionDecision{
			ID:        "D-100",
			Title:     "decisao nova 100",
			Rationale: "teste",
			MadeAt:    first.Session.StartedAt,
		},
	)
	second.OpenQuestions = append(second.OpenQuestions, "pergunta nova (sintetica)")
	if err := WriteSessionState(root, second); err != nil {
		t.Fatalf("segundo write: %v", err)
	}

	events, err := event.ReadEvents(root, event.EventReadOptions{})
	if err != nil {
		t.Fatalf("event.ReadEvents: %v", err)
	}

	stateRenders := 0
	decisionCount := 0
	openQuestionCount := 0
	seenRefs := map[string]bool{}
	for _, e := range events {
		seenRefs[e.Ref] = true
		switch e.Kind {
		case "state_render":
			stateRenders++
		case "decision":
			decisionCount++
		case "open_question":
			openQuestionCount++
		}
	}
	if stateRenders != 2 {
		t.Errorf("esperava 2 state_render events, veio %d", stateRenders)
	}
	// Total events = soma do 1o write (estado inicial vazio -> sampleState) +
// 2o write (sampleState + 2 decisions + 1 open_question novas):
//   1o write: 1 state_render + 1 decision (D-1) + 1 open_question (Q-1) = 3
//   2o write: 1 state_render + 2 decisions (D-99, D-100) + 1 open_question (Q-2) = 4
// Decision events total: 1+2 = 3. OpenQuestion events total: 1+1 = 2.
	if decisionCount != 3 {
		t.Errorf("esperava 3 decision events (D-1, D-99, D-100), veio %d", decisionCount)
	}
	if openQuestionCount != 2 {
		t.Errorf("esperava 2 open_question events (Q-1, Q-2), veio %d", openQuestionCount)
	}
	for _, want := range []string{"D-99", "D-100"} {
		if !seenRefs[want] {
			t.Errorf("evento com ref=%q nao encontrado", want)
		}
	}
}

func TestWriteSessionStateNaoEmiteParaItemsInalterados(t *testing.T) {
	root := t.TempDir()
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}

	s := sampleState()
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("primeiro write: %v", err)
	}
	if err := WriteSessionState(root, s); err != nil {
		t.Fatalf("segundo write (sem mudancas): %v", err)
	}

	events, err := event.ReadEvents(root, event.EventReadOptions{})
	if err != nil {
		t.Fatalf("event.ReadEvents: %v", err)
	}

	stateRenderCount := 0
	secondRenderIdx := -1
	for i, e := range events {
		if e.Kind == "state_render" {
			stateRenderCount++
			if stateRenderCount == 2 {
				secondRenderIdx = i
			}
		}
	}
	if stateRenderCount != 2 {
		t.Errorf("esperava 2 state_render (1 por write), veio %d", stateRenderCount)
	}
	if secondRenderIdx < 0 {
		t.Fatal("nao encontrei o segundo state_render")
	}

	for i := secondRenderIdx + 1; i < len(events); i++ {
		e := events[i]
		// A-37 Etapa 2: kind 'action'/'blocker' removidos de appendStateDiffEvents;
		// agora per-item kinds sao apenas 'decision'/'open_question'. Ambos devem
		// ser silenciados quando o segundo write nao muda nada.
		if e.Kind == "decision" || e.Kind == "open_question" {
			t.Errorf("segundo write sem mudancas nao deveria emitir %s/%s", e.Kind, e.Ref)
		}
	}
}

// TestWriteSessionStateDetectaMudancaDeRationaleEmDecision valida que
// appendStateDiffEvents detecta alteracao de campo em Decisions e emite
// evento 'decision' correspondente. A-37 Etapa 2 removeu os loops de
// action/blocker em appendStateDiffEvents (model.go); o que resta e o
// loop de diffDecisions que detecta alteracoes em Title/Rationale (veja
// diffDecisions em model.go). Substitui o antigo teste que esperava
// mudanca de status em NextActions (campo removido do struct).
func TestWriteSessionStateDetectaMudancaDeStatusEmAction(t *testing.T) {
	root := t.TempDir()
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}

	first := sampleState()
	if err := WriteSessionState(root, first); err != nil {
		t.Fatalf("primeiro write: %v", err)
	}

	second := first
	// sampleState() cria 1 Decision D-1; alterar rationale dispara diffDecisions.
	for i := range second.Decisions {
		if second.Decisions[i].ID == "D-1" {
			second.Decisions[i].Rationale = "rationale alterado em reescrita"
		}
	}
	if err := WriteSessionState(root, second); err != nil {
		t.Fatalf("segundo write: %v", err)
	}

	events, err := event.ReadEvents(root, event.EventReadOptions{})
	if err != nil {
		t.Fatalf("event.ReadEvents: %v", err)
	}
	var found bool
	for _, e := range events {
		// 1o write emite D-1 (criacao); 2o write tambem emite D-1 (alteracao em
		// rationale). Procuramos o evento de rationale_len correspondente ao
		// novo tamanho.
		if e.Ref == "D-1" && e.Kind == "decision" {
			if rl, ok := e.Details["rationale_len"].(float64); ok && rl == float64(len("rationale alterado em reescrita")) {
				found = true
			}
		}
	}
	if !found {
		t.Error("mudanca de rationale em D-1 deveria ter emitido evento decision com rationale_len correto")
	}
}

func TestWriteSessionStateFalhaEmEventNaoAbortaWrite(t *testing.T) {
	root := t.TempDir()
	if err := ensureStateDir(root); err != nil {
		t.Fatal(err)
	}

	invalidEvent := sampleState()
	invalidEvent.SchemaVersion = "9.9"
	if err := WriteSessionState(root, invalidEvent); err == nil {
		t.Fatal("esperava erro de validate em schema_version invalido")
	}

	if _, err := os.Stat(filepath.Join(root, SessionStateDirName, SessionStateFileName)); err == nil {
		t.Error("snapshot nao deveria ter sido escrito apos erro de validate")
	}
}
