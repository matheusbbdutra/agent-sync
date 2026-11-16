package evalrun

import (
	"fmt"
	"sort"
)

// Comparison e o veredito de comparabilidade + regressao de uma execucao
// contra um baseline. Comparable=false significa que os hashes divergem e
// nenhum delta deve ser interpretado (o binario sai com exit 3).
type Comparison struct {
	Comparable    bool              `json:"comparable"`
	Reason        string            `json:"reason,omitempty"`
	Tolerance     float64           `json:"tolerance"`
	DeltaPassRate float64           `json:"pass_rate_delta"`
	Regressions   int               `json:"regressions"`
	Slices        []SliceComparison `json:"slices,omitempty"`
}

// SliceComparison e o delta de um slice. Baseline/Target sao ponteiros porque
// um slice pode existir so de um lado (novo ou removido) — nesse caso nao ha
// delta nem regressao a declarar.
type SliceComparison struct {
	Slice         string   `json:"slice"`
	Baseline      *float64 `json:"baseline_pass_rate,omitempty"`
	Target        *float64 `json:"target_pass_rate,omitempty"`
	DeltaPassRate *float64 `json:"pass_rate_delta,omitempty"`
	NewSlice      bool     `json:"new_slice,omitempty"`
	RemovedSlice  bool     `json:"removed_slice,omitempty"`
	Regression    bool     `json:"regression"`
}

// toleranceEpsilon absorve erro de ponto flutuante na comparacao: uma queda
// de exatamente --tolerance nao pode virar regressao so porque
// 49/100 - 51/100 resulta em -0.020000000000000018.
const toleranceEpsilon = 1e-9

// CompareBaseline compara current com baseline. Suite ou target com hash
// diferente tornam a execucao incomparavel: os casos medidos nao sao os mesmos
// (suite) ou o artefato avaliado mudou de forma a invalidar o numero anterior
// (target). Regressao = queda de pass_rate acima de tolerance (comparacao
// estrita: queda igual a tolerance nao conta).
func CompareBaseline(baseline, current *RunRecord, tolerance float64) Comparison {
	cmp := Comparison{Tolerance: tolerance}
	if baseline == nil || current == nil {
		cmp.Reason = "sem registro de baseline para comparar"
		return cmp
	}
	if baseline.SuiteHash != current.SuiteHash {
		cmp.Reason = fmt.Sprintf("suite_hash diverge (baseline %s, atual %s): a suite mudou, pass_rate nao e comparavel",
			baseline.SuiteHash, current.SuiteHash)
		return cmp
	}
	if baseline.TargetHash != current.TargetHash {
		cmp.Reason = fmt.Sprintf("target_hash diverge (baseline %s, atual %s): o alvo mudou, pass_rate nao e comparavel",
			baseline.TargetHash, current.TargetHash)
		return cmp
	}
	if baseline.Runner != current.Runner {
		cmp.Reason = fmt.Sprintf("runner diverge (baseline %q, atual %q): a saida e gerada por comando diferente, pass_rate nao e comparavel",
			baseline.Runner, current.Runner)
		return cmp
	}
	cmp.Comparable = true

	baseSlices := map[string]SliceStats{}
	for _, st := range baseline.BySlice {
		baseSlices[st.Slice] = st
	}
	targetSlices := map[string]SliceStats{}
	for _, st := range current.BySlice {
		targetSlices[st.Slice] = st
	}

	names := make([]string, 0, len(baseSlices)+len(targetSlices))
	seen := map[string]bool{}
	for name := range baseSlices {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for name := range targetSlices {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		base, inBase := baseSlices[name]
		tgt, inTarget := targetSlices[name]
		sc := SliceComparison{Slice: name}
		switch {
		case inBase && inTarget:
			b, t := base.PassRate, tgt.PassRate
			delta := t - b
			sc.Baseline, sc.Target, sc.DeltaPassRate = &b, &t, &delta
			if delta < -tolerance-toleranceEpsilon {
				sc.Regression = true
				cmp.Regressions++
			}
		case !inBase && inTarget:
			t := tgt.PassRate
			sc.Target = &t
			sc.NewSlice = true
		default:
			b := base.PassRate
			sc.Baseline = &b
			sc.RemovedSlice = true
		}
		cmp.Slices = append(cmp.Slices, sc)
	}

	// Delta global restrito aos slices presentes dos dois lados: comparar
	// universos diferentes produziria numero sem significado.
	var basePass, baseScored, tgtPass, tgtScored float64
	for name, base := range baseSlices {
		tgt, ok := targetSlices[name]
		if !ok {
			continue
		}
		basePass += base.WeightPass
		baseScored += base.WeightScored
		tgtPass += tgt.WeightPass
		tgtScored += tgt.WeightScored
	}
	if baseScored > 0 && tgtScored > 0 {
		cmp.DeltaPassRate = tgtPass/tgtScored - basePass/baseScored
	}
	return cmp
}
