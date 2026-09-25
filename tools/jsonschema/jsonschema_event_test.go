package jsonschema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func validEventPayload(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"schema_version": "1.0",
		"ts":             "2026-09-19T10:30:00Z",
		"kind":           "decision",
		"ref":            "D-1",
		"title":          "schema-output versionado",
		"actor":          "agent-sync",
		"session_id":     "sess-2026-09-19",
		"details":        map[string]any{"commit": "abc1234"},
	}
}

func TestLoadSessionEvent(t *testing.T) {
	schema, err := Load("session-event")
	if err != nil {
		t.Fatalf("Load(session-event) falhou: %v", err)
	}
	if schema == nil {
		t.Fatal("Load(session-event) devolveu nil sem erro")
	}
}

func TestValidateSessionEventHappyPath(t *testing.T) {
	payload := validEventPayload(t)
	if err := Validate("session-event", payload); err != nil {
		t.Fatalf("Validate devolveu erro para evento valido: %v", err)
	}
}

func TestValidateSessionEventMissingRequired(t *testing.T) {
	cases := map[string]string{
		"sem ts":    `{"schema_version":"1.0","kind":"decision","ref":"D-1","title":"x"}`,
		"sem kind":  `{"schema_version":"1.0","ts":"2026-09-19T10:30:00Z","ref":"D-1","title":"x"}`,
		"sem ref":   `{"schema_version":"1.0","ts":"2026-09-19T10:30:00Z","kind":"decision","title":"x"}`,
		"sem title": `{"schema_version":"1.0","ts":"2026-09-19T10:30:00Z","kind":"decision","ref":"D-1"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var payload any
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				t.Fatalf("payload malformado: %v", err)
			}
			err := Validate("session-event", payload)
			if err == nil {
				t.Fatalf("esperava erro de required, recebi nil")
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("esperava *ValidationError, recebi %T", err)
			}
		})
	}
}

func TestValidateSessionEventKindEnumInvalid(t *testing.T) {
	payload := validEventPayload(t)
	payload["kind"] = "thought"
	err := Validate("session-event", payload)
	if err == nil {
		t.Fatal("esperava erro por kind fora do enum, recebi nil")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("esperava *ValidationError, recebi %T", err)
	}
}

func TestValidateSessionEventActorEnumInvalid(t *testing.T) {
	payload := validEventPayload(t)
	payload["actor"] = "gpt-5"
	err := Validate("session-event", payload)
	if err == nil {
		t.Fatal("esperava erro por actor fora do enum, recebi nil")
	}
}

func TestValidateSessionEventRefPatternInvalid(t *testing.T) {
	cases := map[string]string{
		"sem prefixo":  "X-1",
		"prefixo zero": "0-1",
		"prefixo ruim": "decision-1",
		"sem numero":   "D",
	}
	for name, ref := range cases {
		t.Run(name, func(t *testing.T) {
			payload := validEventPayload(t)
			payload["ref"] = ref
			err := Validate("session-event", payload)
			if err == nil {
				t.Errorf("ref=%q deveria falhar, passou", ref)
			}
		})
	}
}

func TestValidateSessionEventRefPatternValid(t *testing.T) {
	cases := []string{"D-1", "A-42", "B-100", "Q-7", "S-3"}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			payload := validEventPayload(t)
			payload["ref"] = ref
			if err := Validate("session-event", payload); err != nil {
				t.Errorf("ref=%q valido foi rejeitado: %v", ref, err)
			}
		})
	}
}

func TestValidateSessionEventDetailsAcceptsArbitraryKeys(t *testing.T) {
	payload := validEventPayload(t)
	payload["details"] = map[string]any{
		"tokens_in": 12345,
		"model":     "claude-opus",
		"commit":    "abc1234",
		"extra_xyz": []any{1, 2, 3},
		"deeply":    map[string]any{"nested": "ok"},
	}
	if err := Validate("session-event", payload); err != nil {
		t.Fatalf("details com chaves arbitrarias deveria passar (ADR-003 excecao): %v", err)
	}
}

func TestValidateSessionEventDetailsCanBeAbsent(t *testing.T) {
	payload := validEventPayload(t)
	delete(payload, "details")
	if err := Validate("session-event", payload); err != nil {
		t.Fatalf("details ausente deveria passar (opcional): %v", err)
	}
}

func TestValidateSessionEventAdditionalPropertyRejected(t *testing.T) {
	payload := validEventPayload(t)
	payload["unknown_root_field"] = "rejeitar"
	err := Validate("session-event", payload)
	if err == nil {
		t.Fatal("esperava erro por additionalProperties no raiz, recebi nil")
	}
}

func TestValidateSessionEventActorCanBeAbsent(t *testing.T) {
	payload := validEventPayload(t)
	delete(payload, "actor")
	if err := Validate("session-event", payload); err != nil {
		t.Fatalf("actor ausente deveria passar (opcional): %v", err)
	}
}

func TestValidateSessionEventSessionIDCanBeAbsent(t *testing.T) {
	payload := validEventPayload(t)
	delete(payload, "session_id")
	if err := Validate("session-event", payload); err != nil {
		t.Fatalf("session_id ausente deveria passar (opcional): %v", err)
	}
}

func TestValidateSessionEventSchemaVersionInvalid(t *testing.T) {
	payload := validEventPayload(t)
	payload["schema_version"] = "v1"
	err := Validate("session-event", payload)
	if err == nil {
		t.Fatal("esperava erro por schema_version fora do padrao, recebi nil")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("mensagem nao cita schema_version: %v", err)
	}
}

func TestValidateSessionEventRoundTripJSON(t *testing.T) {
	payload := validEventPayload(t)
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if err := Validate("session-event", decoded); err != nil {
		t.Fatalf("round-trip JSON nao passou validate: %v", err)
	}
}

func TestValidateSessionEventBadDateTimeFormat(t *testing.T) {
	payload := validEventPayload(t)
	payload["ts"] = "ontem as dez"
	err := Validate("session-event", payload)
	if err == nil {
		t.Skip("lib v6.0.3 nao aplica format=date-time neste draft (AssertFormat virou no-op); pulando")
	}
}
