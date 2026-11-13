package gate

import (
	"fmt"
	"math"
	"testing"

	"github.com/Balanov/held/manifest"
	"github.com/Balanov/held/scores"
)

const contract = "v=1, candidate=cand, baseline=base, metric=success_rate, " +
	"judge=state_check, selection=sel, heldout=hold, candidates=16, alpha=0.05, min_gain=0.01"

func parse(t *testing.T, text string) *manifest.Manifest {
	t.Helper()
	m, err := manifest.Parse(text)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

// build makes a dataset where the candidate wins on the listed items of each
// split and loses on the others, matching the baseline everywhere else.
//
// The baseline alternates one and zero, so a win is an odd index and a loss is
// an even index.
func build(selWins, selLosses, holdWins, holdLosses []int, selN, holdN int) *scores.Dataset {
	d := scores.NewDataset()
	add := func(split, system, item string, v float64) {
		_ = d.Add(scores.Record{Split: split, System: system, Item: item, Score: v})
	}
	apply := func(n int, wins, losses []int) ([]float64, []float64) {
		win := map[int]bool{}
		loss := map[int]bool{}
		for _, i := range wins {
			win[i] = true
		}
		for _, i := range losses {
			loss[i] = true
		}
		base := make([]float64, n)
		cand := make([]float64, n)
		for i := 0; i < n; i++ {
			base[i] = 0
			if i%2 == 0 {
				base[i] = 1
			}
			cand[i] = base[i]
			if win[i] {
				cand[i] = 1
			}
			if loss[i] {
				cand[i] = 0
			}
		}
		return base, cand
	}
	baseSel, candSel := apply(selN, selWins, selLosses)
	baseHold, candHold := apply(holdN, holdWins, holdLosses)
	for i := 0; i < selN; i++ {
		item := fmt.Sprintf("s%03d", i)
		add("sel", "base", item, baseSel[i])
		add("sel", "cand", item, candSel[i])
	}
	for i := 0; i < holdN; i++ {
		item := fmt.Sprintf("h%03d", i)
		add("hold", "base", item, baseHold[i])
		add("hold", "cand", item, candHold[i])
	}
	return d
}

func TestVerdictProven(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3, 5, 7}, nil, []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23}, nil, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Verdict != VerdictProven {
		t.Errorf("verdict = %s, want proven (%+v)", res.Verdict, res)
	}
	if res.LowerBound <= m.MinGain {
		t.Errorf("lower bound %v does not clear %v", res.LowerBound, m.MinGain)
	}
}

func TestVerdictNotProven(t *testing.T) {
	// The selection split reports no gain either, so this is not a conflict.
	m := parse(t, contract)
	d := build(nil, []int{0, 2}, nil, nil, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Verdict != VerdictNotProven {
		t.Errorf("verdict = %s, want not_proven", res.Verdict)
	}
}

func TestVerdictConflicting(t *testing.T) {
	// The selection split reports a gain, the held-out split shows none.
	m := parse(t, contract)
	d := build([]int{1, 3, 5}, nil, []int{1}, []int{0}, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.ClaimedGain <= 0 {
		t.Fatalf("claimed gain = %v, expected a positive claim", res.ClaimedGain)
	}
	if res.Verdict != VerdictConflicting {
		t.Errorf("verdict = %s, want conflicting", res.Verdict)
	}
}

func TestSelectionInflationIsPositive(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3, 5}, nil, []int{1}, []int{0}, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inflation <= 0 {
		t.Errorf("inflation = %v, expected a positive value", res.Inflation)
	}
	if res.CorrectedClaim >= res.ClaimedGain {
		t.Errorf("the corrected claim %v should be below the raw claim %v",
			res.CorrectedClaim, res.ClaimedGain)
	}
}

func TestSingleCandidateHasNoInflation(t *testing.T) {
	m := parse(t, "v=1, candidate=cand, baseline=base, metric=success_rate, "+
		"selection=sel, heldout=hold, candidates=1, alpha=0.05, min_gain=0.01")
	d := build([]int{1, 3, 5}, nil, []int{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23}, nil, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inflation != 0 {
		t.Errorf("inflation = %v, want 0 for a single candidate", res.Inflation)
	}
}

func TestEvaluateIsDeterministic(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3, 5}, nil, []int{1, 3}, []int{0}, 16, 64)
	first, err := Evaluate(m, d)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := Evaluate(m, d)
		if err != nil {
			t.Fatal(err)
		}
		if again.Verdict != first.Verdict ||
			again.ClaimedGain != first.ClaimedGain ||
			again.Inflation != first.Inflation ||
			again.LowerBound != first.LowerBound {
			t.Fatal("the evaluation is not deterministic")
		}
	}
}

func TestEvaluateRequiresBothSplits(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3, 5}, nil, nil, nil, 16, 0)
	if _, err := Evaluate(m, d); err == nil {
		t.Error("expected an error when the held-out split is missing")
	}
}

func TestRequiredHeldoutIsReported(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3, 5}, nil, []int{1, 3}, []int{0}, 16, 64)
	res, err := Evaluate(m, d)
	if err != nil {
		t.Fatal(err)
	}
	if res.RequiredHeldout < 1000 {
		t.Errorf("required held-out items = %d, expected a large number", res.RequiredHeldout)
	}
}

func TestPlanUsesWorstCaseWithoutData(t *testing.T) {
	m := parse(t, contract)
	p := MakePlan(m, 0, nil)
	if p.RequiredHeldout < 30000 {
		t.Errorf("required held-out items = %d", p.RequiredHeldout)
	}
	if p.SigmaSource != "worst case for a binary metric" {
		t.Errorf("sigma source = %q", p.SigmaSource)
	}
}

func TestPlanEstimatesSigmaFromData(t *testing.T) {
	m := parse(t, contract)
	d := build([]int{1, 3}, nil, []int{1, 3, 5, 7}, nil, 16, 64)
	diffs, _, _ := d.Pair("hold", "base", "cand")
	p := MakePlan(m, 16, diffs)
	if p.SigmaSource != "estimated from the held-out split" {
		t.Errorf("sigma source = %q", p.SigmaSource)
	}
}

func TestPlanInflationGrowsWithSelectionSize(t *testing.T) {
	m := parse(t, contract)
	small := MakePlan(m, 16, nil)
	large := MakePlan(m, 256, nil)
	if !(large.ExpectedInflation < small.ExpectedInflation) {
		t.Errorf("inflation should shrink with a larger selection set: %v vs %v",
			large.ExpectedInflation, small.ExpectedInflation)
	}
	if math.Abs(small.ExpectedInflation) < 0.05 {
		t.Errorf("inflation for a 16 item selection = %v, expected a noticeable value",
			small.ExpectedInflation)
	}
}
