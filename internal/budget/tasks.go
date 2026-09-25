package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/token-tools/jsonschema"
)

const (
	AgentTaskSchemaVersion = "1.0"
	AgentTaskFileName      = "agent_tasks.jsonl"
	AgentTaskRotationName  = "agent_tasks.jsonl.1"
	AgentTaskRotateBytes   = 10 * 1024 * 1024
)

type AgentTask struct {
	SchemaVersion string    `json:"schema_version"`
	TaskID        string    `json:"task_id"`
	TS            time.Time `json:"ts"`
	CLI           string    `json:"cli"`
	Model         string    `json:"model"`
	SessionID     string    `json:"session_id,omitempty"`
	TokensIn      *int64    `json:"tokens_in,omitempty"`
	TokensOut     *int64    `json:"tokens_out,omitempty"`
	CostEstimate  *float64  `json:"cost_estimate,omitempty"`
	DurationMs    *int64    `json:"duration_ms,omitempty"`
	Status        string    `json:"status"`
}

func agentTaskPath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, AgentTaskFileName)
}

func agentTaskRotationPath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, AgentTaskRotationName)
}

// AppendAgentTask escreve a task no log append-only .agent-sync/agent_tasks.jsonl.
// Validacao via jsonschema.Validate antes do write; rotacao por tamanho (10MB)
// antes do append se necessario.
func AppendAgentTask(projectRoot string, t AgentTask) error {
	if t.SchemaVersion == "" {
		t.SchemaVersion = AgentTaskSchemaVersion
	}
	if t.TS.IsZero() {
		t.TS = time.Now().UTC()
	}
	if err := jsonschema.Validate("agent_tasks", t); err != nil {
		return fmt.Errorf("agent_task: schema invalido: %w", err)
	}
	if err := pathutil.EnsureStateDir(projectRoot); err != nil {
		return err
	}

	line, err := json.Marshal(&t)
	if err != nil {
		return fmt.Errorf("agent_task: marshal: %w", err)
	}
	line = append(line, '\n')

	if err := rotateAgentTaskIfNeeded(projectRoot, len(line)); err != nil {
		return err
	}

	f, err := os.OpenFile(agentTaskPath(projectRoot), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("agent_task: open %s: %w", agentTaskPath(projectRoot), err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("agent_task: write: %w", err)
	}
	return nil
}

// rotateAgentTaskIfNeeded rotaciona o log se append faria o arquivo
// ultrapassar AgentTaskRotateBytes. Rotacao single-rotation (.1 substitui
// anterior), consistente com padrao session-event.jsonl.
func rotateAgentTaskIfNeeded(projectRoot string, appendBytes int) error {
	info, err := os.Stat(agentTaskPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size()+int64(appendBytes) < AgentTaskRotateBytes {
		return nil
	}
	if err := os.Rename(agentTaskPath(projectRoot), agentTaskRotationPath(projectRoot)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("agent_task: rotacao falhou: %w", err)
	}
	return nil
}

type AgentTaskReadOptions struct {
	Last    int
	CLI     string
	Status  string
	SinceTS time.Time
}

func ReadAgentTasks(projectRoot string, opts AgentTaskReadOptions) ([]AgentTask, error) {
	data, err := os.ReadFile(agentTaskPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []AgentTask
	for i, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var t AgentTask
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			fmt.Fprintf(os.Stderr, "agent_task: linha %d ignorada (invalida): %v\n", i+1, err)
			continue
		}
		if opts.CLI != "" && t.CLI != opts.CLI {
			continue
		}
		if opts.Status != "" && t.Status != opts.Status {
			continue
		}
		if !opts.SinceTS.IsZero() && t.TS.Before(opts.SinceTS) {
			continue
		}
		out = append(out, t)
	}
	if opts.Last > 0 && len(out) > opts.Last {
		out = out[len(out)-opts.Last:]
	}
	return out, nil
}

type AgentTaskStats struct {
	Total             int            `json:"total"`
	ByCLI             map[string]int `json:"by_cli"`
	ByStatus          map[string]int `json:"by_status"`
	TokensInTotal     int64          `json:"tokens_in_total"`
	TokensOutTotal    int64          `json:"tokens_out_total"`
	CostEstimateTotal float64        `json:"cost_estimate_total"`
	FirstTS           time.Time      `json:"first_ts,omitempty"`
	LastTS            time.Time      `json:"last_ts,omitempty"`
	Skipped           int            `json:"skipped_invalid_lines"`
}

func StatsAgentTasks(projectRoot string, opts AgentTaskReadOptions) (AgentTaskStats, error) {
	data, err := os.ReadFile(agentTaskPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return AgentTaskStats{ByCLI: map[string]int{}, ByStatus: map[string]int{}}, nil
		}
		return AgentTaskStats{}, err
	}
	s := AgentTaskStats{
		ByCLI:    map[string]int{},
		ByStatus: map[string]int{},
	}
	for i, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var t AgentTask
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			s.Skipped++
			fmt.Fprintf(os.Stderr, "agent_task stats: linha %d ignorada: %v\n", i+1, err)
			continue
		}
		if opts.CLI != "" && t.CLI != opts.CLI {
			continue
		}
		if opts.Status != "" && t.Status != opts.Status {
			continue
		}
		if !opts.SinceTS.IsZero() && t.TS.Before(opts.SinceTS) {
			continue
		}
		s.Total++
		s.ByCLI[t.CLI]++
		s.ByStatus[t.Status]++
		if t.TokensIn != nil {
			s.TokensInTotal += *t.TokensIn
		}
		if t.TokensOut != nil {
			s.TokensOutTotal += *t.TokensOut
		}
		if t.CostEstimate != nil {
			s.CostEstimateTotal += *t.CostEstimate
		}
		if s.FirstTS.IsZero() || t.TS.Before(s.FirstTS) {
			s.FirstTS = t.TS
		}
		if t.TS.After(s.LastTS) {
			s.LastTS = t.TS
		}
	}
	return s, nil
}
