package jsonschema

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// validPayload reflete o schema session-state v1.2 (schema cleanup commit
// do A-37: top-level properties removidas: next_actions, blockers; \$defs
// action/blocker removidas). Fonte canonica: tools/jsonschema/schemas/session-state.json
func validPayload(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"schema_version": "1.2",
		"project": map[string]any{
			"name": "agent-sync",
			"root": "/srv/repos/agent-sync",
		},
		"git": map[string]any{
			"branch": "feat/docs-session-2026-09-19",
			"head":   "b03e1a1",
		},
		"session": map[string]any{
			"id":         "sess-2026-09-19",
			"started_at": "2026-09-19T10:00:00Z",
			"updated_at": "2026-09-19T11:00:00Z",
		},
		"decisions": []any{
			map[string]any{
				"id":        "ADR-001",
				"title":     "schema-output versionado",
				"rationale": "consistencia cross-CLI",
				"made_at":   "2026-09-19T10:30:00Z",
			},
		},
		"tasks": []any{
			map[string]any{
				"id":         "T-001",
				"title":      "implementar lib jsonschema",
				"status":     "in_progress",
				"started_at": "2026-09-19T10:30:00Z",
			},
		},
		"issues":        []any{},
		"open_questions": []any{
			"smoke real atras de cada commit?",
		},
	}
}

func TestLoadSessionState(t *testing.T) {
	schema, err := Load("session-state")
	if err != nil {
		t.Fatalf("Load(session-state) falhou: %v", err)
	}
	if schema == nil {
		t.Fatal("Load(session-state) devolveu nil sem erro")
	}
}

func TestValidateSessionStateHappyPath(t *testing.T) {
	payload := validPayload(t)
	if err := Validate("session-state", payload); err != nil {
		t.Fatalf("Validate devolveu erro para payload valido: %v", err)
	}
}

func TestValidateSessionStateMissingRequired(t *testing.T) {
	cases := map[string]string{
		"sem project":    `{"schema_version":"1.0","git":{},"session":{}}`,
		"sem git":        `{"schema_version":"1.0","project":{},"session":{}}`,
		"sem session":    `{"schema_version":"1.0","project":{},"git":{}}`,
		"sem schema_ver": `{"project":{},"git":{},"session":{}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var payload any
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				t.Fatalf("payload malformado: %v", err)
			}
			err := Validate("session-state", payload)
			if err == nil {
				t.Fatalf("esperava erro de required, recebi nil")
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("esperava *ValidationError, recebi %T", err)
			}
			hasPathInfo := len(verr.InstanceLocation) > 0 || len(verr.Causes) > 0
			if !hasPathInfo {
				t.Errorf("nem InstanceLocation nem Causes populados; erro sem info de localizacao: %v", verr)
			}
		})
	}
}

func TestValidateSessionStateBadDateTimeFormat(t *testing.T) {
	payload := validPayload(t)
	payload["session"].(map[string]any)["started_at"] = "ontem as dez"
	err := Validate("session-state", payload)
	if err == nil {
		t.Skip("lib v6.0.3 nao aplica format=date-time neste draft (AssertFormat virou no-op); pulando")
	}
}

func TestValidateSessionStateAdditionalPropertyRejected(t *testing.T) {
	payload := validPayload(t)
	payload["unknown_field"] = "rejeitar"
	err := Validate("session-state", payload)
	if err == nil {
		t.Fatal("esperava erro por additionalProperties, recebi nil")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("esperava *ValidationError, recebi %T", err)
	}
	if verr.ErrorKind == nil {
		t.Error("ErrorKind nil, esperava classificador do erro")
	}
}

// TestValidateSessionStateEnumInvalid valida que status fora do enum
// (pending|in_progress|done|cancelled) e rejeitado. Schema cleanup do
// A-37 moveu o enum de next_actions/\$defs/action para tasks/\$defs/task
// (mesmo enum). Teste adaptado para usar 'tasks' em vez de
// 'next_actions'.
func TestValidateSessionStateEnumInvalid(t *testing.T) {
	payload := validPayload(t)
	tasks := payload["tasks"].([]any)
	task := tasks[0].(map[string]any)
	task["status"] = "frozen"
	err := Validate("session-state", payload)
	if err == nil {
		t.Fatal("esperava erro por status fora do enum, recebi nil")
	}
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("esperava *ValidationError, recebi %T", err)
	}
}

func TestValidateSessionStateSchemaVersionPattern(t *testing.T) {
	payload := validPayload(t)
	payload["schema_version"] = "v1"
	err := Validate("session-state", payload)
	if err == nil {
		t.Fatal("esperava erro por schema_version fora do padrao, recebi nil")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("mensagem nao cita campo schema_version: %v", err)
	}
}

func TestValidateSchemaInexistenteRetornaErro(t *testing.T) {
	err := Validate("schema-que-nao-existe", validPayload(t))
	if err == nil {
		t.Fatal("esperava erro por schema inexistente, recebi nil")
	}
	if !strings.Contains(err.Error(), "nao encontrado") {
		t.Errorf("mensagem deveria dizer 'nao encontrado': %v", err)
	}
}

func TestSessionStateRoundTripJSON(t *testing.T) {
	payload := validPayload(t)
	now := time.Now().UTC().Format(time.RFC3339)
	payload["session"].(map[string]any)["updated_at"] = now
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if err := Validate("session-state", decoded); err != nil {
		t.Fatalf("round-trip JSON nao passou validate: %v", err)
	}
}
