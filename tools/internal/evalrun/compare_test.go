package evalrun

import (
	"strings"
	"testing"
)

func rec(slices ...SliceStats) *RunRecord {
	return &RunRecord{
		SchemaVersion: RecordSchemaVersion,
		RunID:         "eval-00000000-000000-0000",
		SuiteHash:     "sha256:aaaaaaaa",
		TargetHash:    "sha256:bbbbbbbb",
		BySlice:       slices,
	}
}

func slice(name string, weightPass, weightScored float64) SliceStats {
	rate := 0.0
	if weightScored > 0 {
		rate = weightPass / weightScored
	}
	return SliceStats{Slice: name, WeightPass: weightPass, WeightScored: weightScored, PassRate: rate}
}

func TestCompareBaselineComputesDelta(t *testing.T) {
	base := rec(slice("s1", 4, 4), slice("s2", 2, 4))
	cur := rec(slice("s1", 4, 4), slice("s2", 1, 4))

	cmp := CompareBaseline(base, cur, 0.02)
	if !cmp.Comparable {
		t.Fatalf("comparavel = false: %s", cmp.Reason)
	}
	if cmp.Regressions != 1 {
		t.Fatalf("regressions = %d, quer 1", cmp.Regressions)
	}
	// base: (4+2)/8 = 0.75 ; atual: (4+1)/8 = 0.625
	if cmp.DeltaPassRate > -0.12 || cmp.DeltaPassRate < -0.13 {
		t.Fatalf("delta = %v, quer ~-0.125", cmp.DeltaPassRate)
	}
	var s2 *SliceComparison
	for i := range cmp.Slices {
		if cmp.Slices[i].Slice == "s2" {
			s2 = &cmp.Slices[i]
		}
	}
	if s2 == nil || !s2.Regression {
		t.Fatalf("slice s2 deveria ser regressao: %+v", s2)
	}
}

func TestCompareBaselineSuiteHashMismatch(t *testing.T) {
	base := rec(slice("s1", 4, 4))
	cur := rec(slice("s1", 4, 4))
	cur.SuiteHash = "sha256:cccccccc"

	cmp := CompareBaseline(base, cur, 0.02)
	if cmp.Comparable {
		t.Fatal("suite_hash diferente deveria tornar a execucao incomparavel")
	}
	if !strings.Contains(cmp.Reason, "suite_hash") {
		t.Fatalf("reason = %q, quer citar suite_hash", cmp.Reason)
	}
}

func TestCompareBaselineTargetHashMismatch(t *testing.T) {
	base := rec(slice("s1", 4, 4))
	cur := rec(slice("s1", 4, 4))
	cur.TargetHash = "sha256:dddddddd"

	cmp := CompareBaseline(base, cur, 0.02)
	if cmp.Comparable {
		t.Fatal("target_hash diferente deveria tornar a execucao incomparavel")
	}
	if !strings.Contains(cmp.Reason, "target_hash") {
		t.Fatalf("reason = %q, quer citar target_hash", cmp.Reason)
	}
}

func TestCompareBaselineTolerance(t *testing.T) {
	// Queda de exatamente 0.02 nao conta como regressao (comparacao estrita).
	base := rec(slice("s1", 51, 100))
	cur := rec(slice("s1", 49, 100))

	if cmp := CompareBaseline(base, cur, 0.02); cmp.Regressions != 0 {
		t.Fatalf("regressions = %d, quer 0 (queda == tolerance)", cmp.Regressions)
	}
	if cmp := CompareBaseline(base, cur, 0.01); cmp.Regressions != 1 {
		t.Fatalf("regressions = %d, quer 1 (queda > tolerance)", cmp.Regressions)
	}
}

func TestCompareBaselineNewAndRemovedSlice(t *testing.T) {
	base := rec(slice("s1", 4, 4), slice("antigo", 1, 2))
	cur := rec(slice("s1", 4, 4), slice("novo", 1, 1))

	cmp := CompareBaseline(base, cur, 0.02)
	if !cmp.Comparable {
		t.Fatalf("comparavel = false: %s", cmp.Reason)
	}
	if cmp.Regressions != 0 {
		t.Fatalf("regressions = %d, quer 0 (slice novo/removido nao tem delta)", cmp.Regressions)
	}
	got := map[string]SliceComparison{}
	for _, sc := range cmp.Slices {
		got[sc.Slice] = sc
	}
	if !got["novo"].NewSlice || got["novo"].Baseline != nil {
		t.Errorf("slice novo = %+v, quer new_slice sem baseline", got["novo"])
	}
	if !got["antigo"].RemovedSlice || got["antigo"].Target != nil {
		t.Errorf("slice antigo = %+v, quer removed_slice sem target", got["antigo"])
	}
}

func TestCompareBaselineNilRecord(t *testing.T) {
	cmp := CompareBaseline(nil, rec(slice("s1", 1, 1)), 0.02)
	if cmp.Comparable || cmp.Reason == "" {
		t.Fatalf("cmp = %+v, quer incomparavel com motivo", cmp)
	}
}
