package evalrun

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var runIDPattern = regexp.MustCompile(`^eval-\d{8}-\d{6}-[0-9a-f]{4}$`)

func TestHashFilePrefixAndStability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alvo.md")
	if err := os.WriteFile(path, []byte("conteudo"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	first, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("hash = %q, quer prefixo sha256:", first)
	}
	if len(first) != len("sha256:")+64 {
		t.Fatalf("hash = %q, quer 64 hex apos o prefixo", first)
	}
	second, _ := HashFile(path)
	if first != second {
		t.Fatalf("hash nao deterministico: %q vs %q", first, second)
	}
	if err := os.WriteFile(path, []byte("conteudo "), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	changed, _ := HashFile(path)
	if changed == first {
		t.Fatal("conteudo diferente deveria gerar hash diferente")
	}
}

func TestNewRunIDFormat(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	id, err := NewRunID(now)
	if err != nil {
		t.Fatalf("NewRunID: %v", err)
	}
	if !runIDPattern.MatchString(id) {
		t.Fatalf("run_id = %q, quer eval-YYYYMMDD-HHMMSS-<4 hex>", id)
	}
	if !strings.HasPrefix(id, "eval-20260203-040506-") {
		t.Fatalf("run_id = %q, quer prefixo temporal eval-20260203-040506-", id)
	}
	other, _ := NewRunID(now)
	if id == other {
		t.Fatalf("dois run_id iguais (%q): o sufixo hex deveria variar", id)
	}
}

func TestAppendRecordIsAppendOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".agent-sync", "eval-runs.jsonl")
	first := RunRecord{SchemaVersion: RecordSchemaVersion, RunID: "eval-1", Totals: Totals{Cases: 2, Scored: 2, Pass: 1, Fail: 1}}
	second := RunRecord{SchemaVersion: RecordSchemaVersion, RunID: "eval-2", Totals: Totals{Cases: 1, Scored: 1, Pass: 1}}

	if err := AppendRecord(path, first); err != nil {
		t.Fatalf("AppendRecord 1: %v", err)
	}
	if err := AppendRecord(path, second); err != nil {
		t.Fatalf("AppendRecord 2: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("linhas = %d, quer 2", len(lines))
	}
	if !strings.Contains(lines[0], `"run_id":"eval-1"`) {
		t.Fatalf("linha 1 = %q, quer o primeiro registro preservado", lines[0])
	}

	records, err := ReadRecords(path)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	if len(records) != 2 || records[0].RunID != "eval-1" || records[1].RunID != "eval-2" {
		t.Fatalf("records = %+v, quer [eval-1 eval-2] na ordem do arquivo", records)
	}
	if records[0].Totals.Pass != 1 || records[0].Totals.Fail != 1 {
		t.Fatalf("totals do primeiro registro nao sobreviveu: %+v", records[0].Totals)
	}
}

func TestReadRecordsMissingFileIsEmpty(t *testing.T) {
	records, err := ReadRecords(filepath.Join(t.TempDir(), "nao-existe.jsonl"))
	if err != nil {
		t.Fatalf("arquivo ausente nao e erro: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %d, quer 0", len(records))
	}
}

func TestResolveBaselineByRunIDAndPath(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "eval-runs.jsonl")
	for _, id := range []string{"eval-a", "eval-b"} {
		if err := AppendRecord(store, RunRecord{SchemaVersion: RecordSchemaVersion, RunID: id}); err != nil {
			t.Fatalf("AppendRecord: %v", err)
		}
	}

	byID, err := ResolveBaseline("eval-a", store)
	if err != nil {
		t.Fatalf("ResolveBaseline por id: %v", err)
	}
	if byID.RunID != "eval-a" {
		t.Fatalf("run_id = %q, quer eval-a", byID.RunID)
	}

	byPath, err := ResolveBaseline(store, store)
	if err != nil {
		t.Fatalf("ResolveBaseline por path: %v", err)
	}
	if byPath.RunID != "eval-b" {
		t.Fatalf("run_id = %q, quer eval-b (ultima linha do arquivo)", byPath.RunID)
	}

	if _, err := ResolveBaseline("eval-inexistente", store); err == nil {
		t.Fatal("run_id inexistente deveria falhar")
	}
}

func TestOverallPassRate(t *testing.T) {
	if got := OverallPassRate(nil); got != 0 {
		t.Fatalf("OverallPassRate(nil) = %v, quer 0", got)
	}
	slices := []SliceStats{slice("s1", 3, 4), slice("s2", 1, 2)}
	// (3+1)/(4+2) = 0.666...
	if got := OverallPassRate(slices); got < 0.666 || got > 0.667 {
		t.Fatalf("OverallPassRate = %v, quer ~0.667", got)
	}
}
