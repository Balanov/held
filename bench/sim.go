// Command bench runs the HELD benchmark harness.
//
// The harness simulates the decision a self-improvement loop actually makes:
// a round produces N candidates, the loop picks the one that scores best on a
// small selection set, and then claims that the candidate is an improvement.
// The truth is known by construction, so the false-accept rate of every
// acceptance rule can be measured instead of argued about.
//
// Two scenarios are run for every configuration:
//
//   - the null scenario, where no candidate is actually better than the
//     baseline. Every acceptance here is a false accept.
//   - the alternative scenario, where every candidate is genuinely better by a
//     known margin. Every rejection here is a false reject.
//
// The harness drives the real gate package, so the numbers describe the
// shipped code and not a reimplementation of it.
//
// The generator is a SplitMix64 stream with an explicit seed. It does not use
// the standard library generator, whose algorithm is not contractually stable
// across Go releases. A published result can therefore be reproduced byte for
// byte on any machine.
//
// Usage:
//
//	go run ./bench --out bench/results
package main

import (
	"fmt"
	"math"

	"github.com/Balanov/held/scores"
	"github.com/Balanov/held/stats"
)

// rng is a SplitMix64 generator.
type rng struct {
	state uint64
}

// newRNG returns a generator with the given seed.
func newRNG(seed uint64) *rng { return &rng{state: seed} }

// next returns the next 64 bits.
func (r *rng) next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// float64 returns a value in [0,1).
func (r *rng) float64() float64 {
	return float64(r.next()>>11) / float64(uint64(1)<<53)
}

// bernoulli draws a binary outcome with probability p.
func (r *rng) bernoulli(p float64) float64 {
	if r.float64() < p {
		return 1
	}
	return 0
}

// config describes one benchmark configuration.
type config struct {
	// SelectionItems is the size of the set used to pick the candidate.
	SelectionItems int
	// HeldoutItems is the size of the set never used for selection.
	HeldoutItems int
	// Candidates is how many candidates the round produced.
	Candidates int
	// TrueDelta is the real advantage of a candidate over the baseline.
	TrueDelta float64
	// MinGain is the smallest gain the acceptance rules insist on.
	MinGain float64
	// Alpha is the significance level.
	Alpha float64
	// Rounds is how many rounds to simulate.
	Rounds int
	// Seed is the generator seed.
	Seed uint64
	// KeepRaw stores the per-item draws so the shipped gate can be cross
	// checked against the harness decision.
	KeepRaw bool
}

// raw holds the per-item draws of one round.
type raw struct {
	baseSel  []float64
	candSel  []float64
	baseHold []float64
	candHold []float64
}

// round is the outcome of one simulated improvement round.
type round struct {
	// WinnerSelectionGain is the gain the loop reports on the selection set.
	WinnerSelectionGain float64
	// WinnerHeldoutGain is the real gain of the winner on the held-out set.
	WinnerHeldoutGain float64
	// SelectionStdErr is the standard error of the selection-set difference.
	SelectionStdErr float64
	// HeldoutStdErr is the standard error of the held-out difference.
	HeldoutStdErr float64
	// Accepted records the decision of each rule, in rule order.
	Accepted []bool
	// draws holds the raw outcomes when the configuration asks for them.
	draws *raw
}

// ruleOrder names the acceptance rules in the order their results are stored.
var ruleOrder = []string{"greedy", "selection-z", "held-out"}

// simulate runs one configuration and returns the collected rounds.
//
// The baseline is drawn once per item and shared by every candidate, so the
// comparison is paired. Item difficulty varies across the set, which is what
// makes a paired difference noisy even when the metric is binary.
func simulate(c config) ([]round, error) {
	r := newRNG(c.Seed)
	out := make([]round, 0, c.Rounds)
	z := stats.TwoSidedZ(c.Alpha)

	for k := 0; k < c.Rounds; k++ {
		pSel := make([]float64, c.SelectionItems)
		for i := range pSel {
			pSel[i] = 0.2 + 0.6*r.float64()
		}
		pHold := make([]float64, c.HeldoutItems)
		for i := range pHold {
			pHold[i] = 0.2 + 0.6*r.float64()
		}

		baseSel := make([]float64, c.SelectionItems)
		for i := range baseSel {
			baseSel[i] = r.bernoulli(pSel[i])
		}
		baseHold := make([]float64, c.HeldoutItems)
		for i := range baseHold {
			baseHold[i] = r.bernoulli(pHold[i])
		}

		// Every candidate is measured on the selection set; the loop keeps the
		// one that scores best.
		bestMean := math.Inf(-1)
		var bestCandSel []float64
		for cnd := 0; cnd < c.Candidates; cnd++ {
			cand := make([]float64, c.SelectionItems)
			sum := 0.0
			for i := range cand {
				outcome := r.bernoulli(clamp01(pSel[i] + c.TrueDelta))
				cand[i] = outcome
				sum += outcome - baseSel[i]
			}
			mean := sum / float64(len(cand))
			if mean > bestMean {
				bestMean = mean
				bestCandSel = cand
			}
		}
		if bestCandSel == nil {
			return nil, fmt.Errorf("no candidates")
		}

		// The winner is now measured on the held-out set, which never took part
		// in the selection.
		candHold := make([]float64, c.HeldoutItems)
		sum := 0.0
		for i := range candHold {
			outcome := r.bernoulli(clamp01(pHold[i] + c.TrueDelta))
			candHold[i] = outcome
			sum += outcome - baseHold[i]
		}

		selDiff := make([]float64, len(bestCandSel))
		for i := range selDiff {
			selDiff[i] = bestCandSel[i] - baseSel[i]
		}
		holdDiff := make([]float64, len(candHold))
		for i := range holdDiff {
			holdDiff[i] = candHold[i] - baseHold[i]
		}

		selMean, selSE := meanAndSE(selDiff)
		holdMean, holdSE := meanAndSE(holdDiff)

		rd := round{
			WinnerSelectionGain: selMean,
			WinnerHeldoutGain:   holdMean,
			SelectionStdErr:     selSE,
			HeldoutStdErr:       holdSE,
		}
		rd.Accepted = decideWithZ(c, selMean, selSE, holdMean, holdSE, z)
		if c.KeepRaw {
			rd.draws = &raw{
				baseSel:  baseSel,
				candSel:  bestCandSel,
				baseHold: baseHold,
				candHold: candHold,
			}
		}
		out = append(out, rd)
	}
	return out, nil
}

// decide applies every acceptance rule to one round.
func decide(c config, selMean, selSE, holdMean, holdSE float64) []bool {
	z := stats.TwoSidedZ(c.Alpha)
	return decideWithZ(c, selMean, selSE, holdMean, holdSE, z)
}

// decideWithZ applies every acceptance rule with a precomputed critical value.
func decideWithZ(c config, selMean, selSE, holdMean, holdSE, z float64) []bool {
	return []bool{
		// Greedy: keep the candidate if it scored better on the selection set.
		selMean > c.MinGain,
		// Selection-z: a significance test on the selection set itself. It is
		// invalid after a selection, because the selected score is not a fresh
		// draw; the benchmark measures by how much.
		selMean-z*selSE > c.MinGain,
		// Held-out: the rule HELD implements.
		holdMean-z*holdSE > c.MinGain,
	}
}

// buildDataset turns the raw draws of one round into a score dataset.
func buildDataset(c config, rd raw) (*scores.Dataset, error) {
	d := scores.NewDataset()
	add := func(split, system, item string, v float64) error {
		return d.Add(scores.Record{Split: split, System: system, Item: item, Score: v})
	}
	for i := range rd.baseSel {
		if err := add("sel", "base", fmt.Sprintf("s%04d", i), rd.baseSel[i]); err != nil {
			return nil, err
		}
		if err := add("sel", "cand", fmt.Sprintf("s%04d", i), rd.candSel[i]); err != nil {
			return nil, err
		}
	}
	for i := range rd.baseHold {
		if err := add("hold", "base", fmt.Sprintf("h%04d", i), rd.baseHold[i]); err != nil {
			return nil, err
		}
		if err := add("hold", "cand", fmt.Sprintf("h%04d", i), rd.candHold[i]); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// meanAndSE returns the mean and the standard error of the mean.
func meanAndSE(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	m := sum / float64(len(xs))
	if len(xs) < 2 {
		return m, 0
	}
	ss := 0.0
	for _, x := range xs {
		d := x - m
		ss += d * d
	}
	sd := math.Sqrt(ss / float64(len(xs)-1))
	return m, sd / math.Sqrt(float64(len(xs)))
}

// clamp01 keeps a probability inside [0,1].
func clamp01(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}
