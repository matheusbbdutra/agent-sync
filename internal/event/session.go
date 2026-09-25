package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/token-tools/jsonschema"
)

const (
	SessionEventSchemaVersion = "1.0"
	SessionEventFileName      = "session-event.jsonl"
	SessionEventRotationName  = "session-event.jsonl.1"
	SessionEventRotateBytes   = 10 * 1024 * 1024
)

type SessionEvent struct {
	SchemaVersion string                 `json:"schema_version"`
	TS            time.Time              `json:"ts"`
	Kind          string                 `json:"kind"`
	Ref           string                 `json:"ref"`
	Title         string                 `json:"title"`
	Actor         string                 `json:"actor,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

func eventPath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, SessionEventFileName)
}

func eventRotationPath(projectRoot string) string {
	return filepath.Join(projectRoot, pathutil.SessionStateDirName, SessionEventRotationName)
}

// AppendEvent grava o evento no log append-only (.agent-sync/session-event.jsonl)
// e espelha no memory-mcp (libSQL).
func AppendEvent(projectRoot string, e SessionEvent) error {
	if e.SchemaVersion == "" {
		e.SchemaVersion = SessionEventSchemaVersion
	}
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	if err := jsonschema.Validate("session-event", e); err != nil {
		return fmt.Errorf("event: schema invalido: %w", err)
	}
	if err := pathutil.EnsureStateDir(projectRoot); err != nil {
		return err
	}

	line, err := json.Marshal(&e)
	if err != nil {
		return fmt.Errorf("event: marshal: %w", err)
	}
	line = append(line, '\n')

	if err := rotateIfNeeded(projectRoot, len(line)); err != nil {
		return err
	}

	f, err := os.OpenFile(eventPath(projectRoot), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("event: open %s: %w", eventPath(projectRoot), err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("event: write: %w", err)
	}

	mirrorEventToMemory(projectRoot, e)
	return nil
}

func rotateIfNeeded(projectRoot string, appendBytes int) error {
	info, err := os.Stat(eventPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size()+int64(appendBytes) < SessionEventRotateBytes {
		return nil
	}
	if err := os.Rename(eventPath(projectRoot), eventRotationPath(projectRoot)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("event: rotacao falhou: %w", err)
	}
	return nil
}

type EventReadOptions struct {
	Last  int
	Kind  string
	Since time.Time
}

func ReadEvents(projectRoot string, opts EventReadOptions) ([]SessionEvent, error) {
	args := ListEventsArgs{Limit: 500}
	if opts.Kind != "" {
		args.Kind = opts.Kind
	}
	if !opts.Since.IsZero() {
		args.Since = opts.Since.UTC().Format(time.RFC3339)
	}
	if projectRoot != "" {
		args.ProjectPath = projectRoot
	}
	rows, err := callListEvents(args, eventStoreTimeout())
	if err != nil || errors.Is(err, ErrEventStoreUnavailable) {
		return readLegacyJSONL(projectRoot, opts)
	}
	var out []SessionEvent
	for _, r := range rows {
		if projectRoot != "" && r.ProjectPath != "" && r.ProjectPath != projectRoot {
			continue
		}
		ev := memoryRowToSessionEvent(r)
		if !opts.Since.IsZero() && ev.TS.Before(opts.Since) {
			continue
		}
		out = append(out, ev)
	}
	if len(out) == 0 {
		return readLegacyJSONL(projectRoot, opts)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	if opts.Last > 0 && len(out) > opts.Last {
		out = out[len(out)-opts.Last:]
	}
	return out, nil
}

type EventStats struct {
	Total   int            `json:"total"`
	ByKind  map[string]int `json:"by_kind"`
	ByActor map[string]int `json:"by_actor"`
	FirstTS time.Time      `json:"first_ts,omitempty"`
	LastTS  time.Time      `json:"last_ts,omitempty"`
	Skipped int            `json:"skipped_invalid_lines"`
}

func StatsEvents(projectRoot string, opts EventReadOptions) (EventStats, error) {
	events, err := ReadEvents(projectRoot, opts)
	if err != nil {
		return EventStats{}, err
	}
	s := EventStats{
		ByKind:  map[string]int{},
		ByActor: map[string]int{},
	}
	for _, e := range events {
		s.Total++
		s.ByKind[e.Kind]++
		if e.Actor != "" {
			s.ByActor[e.Actor]++
		}
		if s.FirstTS.IsZero() || e.TS.Before(s.FirstTS) {
			s.FirstTS = e.TS
		}
		if e.TS.After(s.LastTS) {
			s.LastTS = e.TS
		}
	}
	return s, nil
}

func FilteredEvents(events []SessionEvent, opts EventReadOptions) []SessionEvent {
	var out []SessionEvent
	for _, e := range events {
		if opts.Kind != "" && e.Kind != opts.Kind {
			continue
		}
		if !opts.Since.IsZero() && e.TS.Before(opts.Since) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	if opts.Last > 0 && len(out) > opts.Last {
		out = out[len(out)-opts.Last:]
	}
	return out
}
