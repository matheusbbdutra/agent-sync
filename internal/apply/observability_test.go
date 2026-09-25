package apply

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHookEventsHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	content := "" +
		`{"timestamp":"t1","event":"hook_error","version":"1","stage":"PostToolUse","code":"rc=1","message":"a","cli":"codex","tool":"bash","session_id":"s1"}` + "\n" +
		`{"timestamp":"t2","event":"hook_error","version":"1","stage":"PostToolUse","code":"rc=2","message":"b","cli":"codex","tool":"read","session_id":"s1"}` + "\n" +
		`{"timestamp":"t3","event":"hook_error","version":"1","stage":"PreToolUse","code":"rc=3","message":"c","cli":"codex","tool":"","session_id":"s2"}` + "\n" +
		`{"timestamp":"t4","event":"hook_error","version":"1","stage":"Stop","code":"rc=4","message":"d","cli":"claude","tool":"","session_id":""}` + "\n" +
		`{"timestamp":"t5","event":"hook_error","version":"1","stage":"PostToolUse","code":"rc=5","message":"e","cli":"codex","tool":"write","session_id":"s3"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readHookEvents(path)
	if err != nil {
		t.Fatalf("readHookEvents: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	wantStages := []string{"PostToolUse", "PostToolUse", "PreToolUse", "Stop", "PostToolUse"}
	wantCodes := []string{"rc=1", "rc=2", "rc=3", "rc=4", "rc=5"}
	for i, ev := range events {
		if ev.Stage != wantStages[i] || ev.Code != wantCodes[i] {
			t.Errorf("event[%d]=%+v want stage=%s code=%s", i, ev, wantStages[i], wantCodes[i])
		}
	}
}

func TestReadHookEventsSkipsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	content := "" +
		`{"stage":"PostToolUse","code":"rc=1"}` + "\n" +
		`{this is not json}` + "\n" +
		`{"stage":"PreToolUse","code":"rc=2"}` + "\n" +
		`{"stage"` + "\n" +
		`{"stage":"Stop","code":"rc=3"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readHookEvents(path)
	if err != nil {
		t.Fatalf("readHookEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 valid events (2 invalid lines skipped silently), got %d", len(events))
	}
	wantStages := []string{"PostToolUse", "PreToolUse", "Stop"}
	wantCodes := []string{"rc=1", "rc=2", "rc=3"}
	for i, ev := range events {
		if ev.Stage != wantStages[i] || ev.Code != wantCodes[i] {
			t.Errorf("event[%d]=%+v want stage=%s code=%s", i, ev, wantStages[i], wantCodes[i])
		}
	}
}

func TestReadHookEventsMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.jsonl")
	events, err := readHookEvents(path)
	if err != nil {
		t.Fatalf("readHookEvents returned error on missing file: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected empty slice for missing file, got %d events: %+v", len(events), events)
	}
}

func TestReadHookEventsRespectsVersionField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	content := `{"timestamp":"t","event":"hook_error","version":"1","stage":"PostToolUse","code":"rc=1","message":"x","cli":"codex","tool":"","session_id":""}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readHookEvents(path)
	if err != nil {
		t.Fatalf("readHookEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Stage != "PostToolUse" || events[0].Code != "rc=1" {
		t.Errorf("event=%+v want stage=PostToolUse code=rc=1 (version field must be tolerated, not break parse)", events[0])
	}
}

func TestReadHookEventsParsesNewFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	dur := 42
	content := "" +
		`{"timestamp":"t1","event":"hook_error","version":"1","stage":"PostToolUse","code":"rc=1","message":"a","cli":"codex","tool":"bash","session_id":"s1","status":"ok","duration_ms":42}` + "\n" +
		`{"timestamp":"t2","event":"hook_error","version":"1","stage":"PreToolUse","code":"rc=2","message":"b","cli":"antigravity","tool":"","session_id":"s2","status":"fail","duration_ms":7}` + "\n" +
		`{"timestamp":"t3","event":"hook_error","version":"1","stage":"Stop","code":"rc=3","message":"c","cli":"cursor","tool":"edit","session_id":""}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := readHookEvents(path)
	if err != nil {
		t.Fatalf("readHookEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	wantCLIs := []string{"codex", "antigravity", "cursor"}
	wantTools := []string{"bash", "", "edit"}
	wantDur := []int{42, 7, 0}
	for i, ev := range events {
		if ev.CLI != wantCLIs[i] {
			t.Errorf("event[%d] CLI=%q want %q", i, ev.CLI, wantCLIs[i])
		}
		if ev.Tool != wantTools[i] {
			t.Errorf("event[%d] Tool=%q want %q", i, ev.Tool, wantTools[i])
		}
		if wantDur[i] == 0 {
			if ev.DurationMs != nil {
				t.Errorf("event[%d] DurationMs=%v want nil", i, ev.DurationMs)
			}
		} else {
			if ev.DurationMs == nil {
				t.Errorf("event[%d] DurationMs=nil want %d", i, wantDur[i])
			} else if *ev.DurationMs != wantDur[i] {
				t.Errorf("event[%d] DurationMs=%d want %d", i, *ev.DurationMs, wantDur[i])
			}
		}
	}

	_ = dur
}

func TestAggregateObservability(t *testing.T) {
	dur := func(v int) *int { return &v }
	events := []hookErrorEvent{
		{Stage: "PostToolUse", Code: "hook_exit_1", CLI: "codex", Tool: "Bash", DurationMs: dur(10)},
		{Stage: "PostToolUse", Code: "hook_exit_1", CLI: "codex", Tool: "Bash", DurationMs: dur(5)},
		{Stage: "PostToolUse", Code: "hook_exit_2", CLI: "codex", Tool: "Write", DurationMs: dur(20)},
		{Stage: "Stop", Code: "false_success_guard_failed", CLI: "antigravity", Tool: "", DurationMs: dur(30)},
		{Stage: "Stop", Code: "false_success_guard_failed", CLI: "antigravity", Tool: "", DurationMs: dur(40)},
		{Stage: "PreToolUse", Code: "protect_mcp_exit_2", CLI: "cursor", Tool: "Bash", DurationMs: dur(50)},
		{Stage: "PostToolUse", Code: "hook_exit_1", CLI: "codex", Tool: "Read", DurationMs: dur(2)},
		{Stage: "PostToolUse", Code: "hook_exit_1", CLI: "antigravity", Tool: "Bash", DurationMs: dur(60)},
		{Stage: "PreToolUse", Code: "protect_mcp_exit_2", CLI: "cursor", Tool: "Write", DurationMs: nil},
		{Stage: "Stop", Code: "false_success_guard_failed", CLI: "cursor", Tool: "", DurationMs: nil},
	}

	summary := aggregateHookEvents(events)

	if got, want := summary.ByStageCode, []keyCount{
		{Key: "PostToolUse:hook_exit_1", Count: 4},
		{Key: "Stop:false_success_guard_failed", Count: 3},
		{Key: "PreToolUse:protect_mcp_exit_2", Count: 2},
		{Key: "PostToolUse:hook_exit_2", Count: 1},
	}; !equalKeyCounts(got, want) {
		t.Errorf("ByStageCode=%v want %v", got, want)
	}

	if got, want := summary.ByCLI, []keyCount{
		{Key: "codex", Count: 4},
		{Key: "antigravity", Count: 3},
		{Key: "cursor", Count: 3},
	}; !equalKeyCounts(got, want) {
		t.Errorf("ByCLI=%v want %v", got, want)
	}

	if got, want := summary.ByTool, []keyCount{
		{Key: "Bash", Count: 4},
		{Key: "(sem tool)", Count: 3},
		{Key: "Write", Count: 2},
		{Key: "Read", Count: 1},
	}; !equalKeyCounts(got, want) {
		t.Errorf("ByTool=%v want %v", got, want)
	}

	if summary.LatencyMs == nil {
		t.Fatal("LatencyMs is nil, want populated")
	}
	if summary.LatencyMs.Samples != 8 {
		t.Errorf("LatencyMs.Samples=%d want 8", summary.LatencyMs.Samples)
	}
	if summary.LatencyMs.Max != 60 {
		t.Errorf("LatencyMs.Max=%d want 60", summary.LatencyMs.Max)
	}
	if summary.LatencyMs.P50 < 1 || summary.LatencyMs.P50 > 60 {
		t.Errorf("LatencyMs.P50=%d out of range", summary.LatencyMs.P50)
	}
	if summary.LatencyMs.P95 < 50 || summary.LatencyMs.P95 > 60 {
		t.Errorf("LatencyMs.P95=%d out of range", summary.LatencyMs.P95)
	}
}

func TestPercentile(t *testing.T) {
	cases := []struct {
		name   string
		values []int
		p      float64
		want   int
	}{
		{name: "empty", values: nil, p: 0.5, want: 0},
		{name: "single", values: []int{42}, p: 0.5, want: 42},
		{name: "p out of range low", values: []int{1, 2, 3}, p: -0.1, want: 0},
		{name: "p out of range high", values: []int{1, 2, 3}, p: 1.5, want: 0},
		{name: "odd median", values: []int{10, 20, 30, 40, 50}, p: 0.5, want: 30},
		{name: "odd p95", values: []int{10, 20, 30, 40, 50}, p: 0.95, want: 50},
		{name: "even median", values: []int{10, 20, 30, 40}, p: 0.5, want: 20},
		{name: "p0", values: []int{10, 20, 30}, p: 0, want: 10},
		{name: "p1", values: []int{10, 20, 30}, p: 1, want: 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percentile(tc.values, tc.p); got != tc.want {
				t.Errorf("percentile(%v, %v)=%d want %d", tc.values, tc.p, got, tc.want)
			}
		})
	}
}

func TestPrintHookObservabilityJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	content := "" +
		`{"timestamp":"t1","event":"hook_error","version":"1","stage":"PostToolUse","code":"hook_exit_1","message":"a","cli":"codex","tool":"Bash","session_id":"s1","status":"ok","duration_ms":4}` + "\n" +
		`{"timestamp":"t2","event":"hook_error","version":"1","stage":"PostToolUse","code":"hook_exit_1","message":"b","cli":"codex","tool":"Write","session_id":"s2","status":"fail","duration_ms":6}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_HOOK_LOG", path)

	var buf bytes.Buffer
	if err := printHookObservabilityTo(&buf, true); err != nil {
		t.Fatalf("printHookObservabilityTo json: %v", err)
	}

	var got observabilityReport
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%s", err, buf.String())
	}
	if got.Path != path {
		t.Errorf("Path=%q want %q", got.Path, path)
	}
	if got.Total != 2 {
		t.Errorf("Total=%d want 2", got.Total)
	}
	if len(got.ByStageCode) != 1 || got.ByStageCode[0].Key != "PostToolUse:hook_exit_1" || got.ByStageCode[0].Count != 2 {
		t.Errorf("ByStageCode=%+v want single entry with count=2", got.ByStageCode)
	}
	if len(got.ByCLI) != 1 || got.ByCLI[0].Key != "codex" || got.ByCLI[0].Count != 2 {
		t.Errorf("ByCLI=%+v want single entry codex count=2", got.ByCLI)
	}
	if len(got.ByTool) != 2 {
		t.Errorf("ByTool=%+v want 2 entries", got.ByTool)
	}
	if got.LatencyMs == nil || got.LatencyMs.Samples != 2 || got.LatencyMs.Max != 6 {
		t.Errorf("LatencyMs=%+v want Samples=2 Max=6", got.LatencyMs)
	}
}

func TestPrintHookObservabilityHumanReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "errors.jsonl")
	content := "" +
		`{"timestamp":"t1","event":"hook_error","version":"1","stage":"PostToolUse","code":"hook_exit_1","message":"a","cli":"codex","tool":"Bash","session_id":"s1","status":"ok","duration_ms":4}` + "\n" +
		`{"timestamp":"t2","event":"hook_error","version":"1","stage":"Stop","code":"false_success_guard_failed","message":"b","cli":"antigravity","tool":"","session_id":"s2","status":"fail","duration_ms":12}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_SYNC_HOOK_LOG", path)

	var buf bytes.Buffer
	if err := printHookObservabilityTo(&buf, false); err != nil {
		t.Fatalf("printHookObservabilityTo text: %v", err)
	}

	out := buf.String()
	wantSections := []string{
		"📊 Erros de hooks",
		"Por stage:code:",
		"Por cli:",
		"Por tool:",
		"Latência",
	}
	for _, s := range wantSections {
		if !strings.Contains(out, s) {
			t.Errorf("output missing section %q\noutput=%s", s, out)
		}
	}
	if strings.Contains(out, "Taxa de sucesso") {
		t.Errorf("output unexpectedly contains removed section %q\noutput=%s", "Taxa de sucesso", out)
	}
	if !strings.Contains(out, "Bash") {
		t.Errorf("output missing tool name Bash\noutput=%s", out)
	}
	if !strings.Contains(out, "(sem tool)") {
		t.Errorf("output missing '(sem tool)' bucket for empty-tool events\noutput=%s", out)
	}
}

func equalKeyCounts(a, b []keyCount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
