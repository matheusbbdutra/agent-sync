package evalrun

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// RecordSchemaVersion versiona o formato de eval-runs.jsonl.
	RecordSchemaVersion = 1
	// DefaultRecordPath e o registro append-only relativo ao cwd.
	DefaultRecordPath = ".agent-sync/eval-runs.jsonl"
)

// Totals e o placar da execucao inteira. Skipped fica fora de Scored, entao
// pass_rate nunca conta caso nao avaliado como acerto.
type Totals struct {
	Cases   int `json:"cases"`
	Scored  int `json:"scored"`
	Skipped int `json:"skipped"`
	Pass    int `json:"pass"`
	Fail    int `json:"fail"`
}

// SliceStats guarda o placar por slice. WeightPass/WeightScored ficam no
// registro para que o pass_rate ponderado seja auditavel sem recalcular.
type SliceStats struct {
	Slice        string  `json:"slice"`
	Cases        int     `json:"cases"`
	Scored       int     `json:"scored"`
	Skipped      int     `json:"skipped"`
	Pass         int     `json:"pass"`
	Fail         int     `json:"fail"`
	WeightPass   float64 `json:"weight_pass"`
	WeightScored float64 `json:"weight_scored"`
	PassRate     float64 `json:"pass_rate"`
}

// BaselineRef identifica o registro usado como baseline.
type BaselineRef struct {
	RunID    string  `json:"run_id"`
	TS       string  `json:"ts"`
	PassRate float64 `json:"pass_rate"`
}

// Delta e a variacao medida contra o baseline.
type Delta struct {
	PassRate    float64 `json:"pass_rate"`
	Regressions int     `json:"regressions"`
}

// RunRecord e uma linha de .agent-sync/eval-runs.jsonl.
type RunRecord struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	TS            string       `json:"ts"`
	Suite         string       `json:"suite"`
	SuiteHash     string       `json:"suite_hash"`
	Target        string       `json:"target"`
	TargetHash    string       `json:"target_hash"`
	Model         string       `json:"model"`
	Runner        string       `json:"runner,omitempty"`
	Totals        Totals       `json:"totals"`
	BySlice       []SliceStats `json:"by_slice"`
	Baseline      *BaselineRef `json:"baseline,omitempty"`
	Delta         *Delta       `json:"delta,omitempty"`
	Comparable    *bool        `json:"comparable,omitempty"`
}

// HashFile devolve "sha256:<hex>" do conteudo do arquivo. O prefixo deixa
// explicito qual algoritmo gerou o hash no registro.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash de %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// NewRunID gera o identificador de execucao no formato
// eval-YYYYMMDD-HHMMSS-<4 hex> (timestamp local + 2 bytes aleatorios).
func NewRunID(now time.Time) (string, error) {
	buf := make([]byte, 2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gerar run_id: %w", err)
	}
	return fmt.Sprintf("eval-%s-%s", now.Format("20060102-150405"), hex.EncodeToString(buf)), nil
}

// AppendRecord acrescenta uma linha JSON ao registro (cria o diretorio pai).
// Append-only: nenhum byte existente e reescrito.
func AppendRecord(path string, rec RunRecord) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("criar %s: %w", dir, err)
		}
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("serializar registro: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("abrir %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("gravar em %s: %w", path, err)
	}
	return nil
}

// ReadRecords le todos os registros na ordem do arquivo. Arquivo ausente
// devolve lista vazia (primeira execucao nao e erro).
func ReadRecords(path string) ([]RunRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var records []RunRecord
	dec := json.NewDecoder(f)
	for {
		var rec RunRecord
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("registro invalido em %s: %w", path, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// FindRecord busca o registro mais recente com o run_id informado.
func FindRecord(records []RunRecord, runID string) (*RunRecord, bool) {
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].RunID == runID {
			return &records[i], true
		}
	}
	return nil, false
}

// ResolveBaseline aceita um caminho de arquivo de registros ou um run_id. No
// caso de arquivo, usa a ultima linha; no caso de run_id, busca no registro
// corrente. Devolve o registro encontrado ou erro explicito.
func ResolveBaseline(value, recordPath string) (*RunRecord, error) {
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		records, err := ReadRecords(value)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("baseline %s: nenhum registro no arquivo", value)
		}
		return &records[len(records)-1], nil
	}
	records, err := ReadRecords(recordPath)
	if err != nil {
		return nil, err
	}
	if rec, ok := FindRecord(records, value); ok {
		return rec, nil
	}
	return nil, fmt.Errorf("baseline %q: nao e arquivo nem run_id presente em %s", value, recordPath)
}

// OverallPassRate calcula o pass_rate ponderado de um conjunto de slices.
// Slices sem caso scored nao entram no denominador.
func OverallPassRate(slices []SliceStats) float64 {
	var weightPass, weightScored float64
	for _, st := range slices {
		weightPass += st.WeightPass
		weightScored += st.WeightScored
	}
	if weightScored == 0 {
		return 0
	}
	return weightPass / weightScored
}
