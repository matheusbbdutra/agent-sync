package jsonschema

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validTaskPayload(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"schema_version": "1.0",
		"task_id":        "task-2026-09-19-001",
		"ts":             "2026-09-19T18:30:00Z",
		"cli":            "claude",
		"model":          "claude-sonnet-4.5",
		"session_id":     "sess-20260919-183000",
		"tokens_in":      1234,
		"tokens_out":     567,
		"cost_estimate":  0.0234,
		"duration_ms":    5000,
		"status":         "completed",
	}
}

func TestLoadAgentTasks(t *testing.T) {
	schema, err := Load("agent_tasks")
	if err != nil {
		t.Fatalf("Load(agent_tasks) falhou: %v", err)
	}
	if schema == nil {
		t.Fatal("schema nil")
	}
}

func TestValidateAgentTasksHappyPath(t *testing.T) {
	if err := Validate("agent_tasks", validTaskPayload(t)); err != nil {
		t.Fatalf("payload valido rejeitado: %v", err)
	}
}

func TestValidateAgentTasksMinimalPayload(t *testing.T) {
	// Apenas required: schema_version, task_id, ts, cli, model, status.
	minimal := map[string]any{
		"schema_version": "1.0",
		"task_id":        "t-1",
		"ts":             "2026-09-19T18:30:00Z",
		"cli":            "cursor",
		"model":          "gpt-5",
		"status":         "started",
	}
	if err := Validate("agent_tasks", minimal); err != nil {
		t.Fatalf("payload minimo rejeitado: %v", err)
	}
}

func TestValidateAgentTasksRejectsMissingRequired(t *testing.T) {
	cases := []string{"task_id", "ts", "cli", "model", "status"}
	for _, field := range cases {
		t.Run(field, func(t *testing.T) {
			payload := validTaskPayload(t)
			delete(payload, field)
			if err := Validate("agent_tasks", payload); err == nil {
				t.Fatalf("campo %q ausente deveria ser rejeitado", field)
			} else if !strings.Contains(err.Error(), field) {
				t.Errorf("erro nao cita campo %q: %v", field, err)
			}
		})
	}
}

func TestValidateAgentTasksRejectsInvalidCLI(t *testing.T) {
	payload := validTaskPayload(t)
	payload["cli"] = "gpt-cli"
	if err := Validate("agent_tasks", payload); err == nil {
		t.Fatal("CLI fora do enum deveria ser rejeitado")
	}
}

func TestValidateAgentTasksRejectsInvalidStatus(t *testing.T) {
	payload := validTaskPayload(t)
	payload["status"] = "running"
	if err := Validate("agent_tasks", payload); err == nil {
		t.Fatal("status fora do enum deveria ser rejeitado")
	}
}

func TestValidateAgentTasksRejectsNegativeTokens(t *testing.T) {
	payload := validTaskPayload(t)
	payload["tokens_in"] = -1
	if err := Validate("agent_tasks", payload); err == nil {
		t.Fatal("tokens_in negativo deveria ser rejeitado")
	}
}

func TestValidateAgentTasksAdditionalPropertyRejected(t *testing.T) {
	payload := validTaskPayload(t)
	payload["extra_field"] = "qualquer"
	if err := Validate("agent_tasks", payload); err == nil {
		t.Fatal("chave adicional no root deveria ser rejeitada (additionalProperties: false)")
	}
}

func TestValidateAgentTasksRoundTripJSON(t *testing.T) {
	payload := validTaskPayload(t)
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal falhou: %v", err)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal falhou: %v", err)
	}
	if err := Validate("agent_tasks", decoded); err != nil {
		t.Fatalf("round-trip JSON nao passou validate: %v", err)
	}
}
