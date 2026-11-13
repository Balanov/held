// Package gate decides whether a claimed improvement is real.
//
// The decision rests on two independent measurements of the same candidate
// against the same baseline:
//
//   - the selection split, the items that were used to pick the candidate out
//     of the round. Its mean difference is the number a greedy loop reports,
//     and it is inflated by selection: the candidate was the best of several
//     noisy draws, so its score sits above the truth by roughly one standard
//     error times the expected maximum of that many draws.
//
//   - the held-out split, the items that were never used for selection. Its
//     mean difference is an unbiased estimate of the true gain, with a
//     confidence interval that says how much of it the data can actually
//     support.
//
// The verdict is taken from the held-out interval alone: the candidate is
// accepted only when the lower end of the interval clears the declared
// minimum gain. A candidate that looks better on the selection split but does
// not clear the held-out bar is reported as conflicting, because that is the
// exact shape of a loop that has fooled itself.
package gate

import (
	"fmt"
	"math"

	"github.com/Balanov/held/manifest"
	"github.com/Balanov/held/scores"
	"github.com/Balanov/held/stats"
)

// Verdict values.
const (
	// VerdictProven means the held-out interval clears the minimum gain.
	VerdictProven = "proven"
	// VerdictNotProven means the held-out interval does not clear it.
	VerdictNotProven = "not_proven"
	// VerdictConflicting means the selection split reports a gain that the
	// held-out split cannot confirm.
	VerdictConflicting = "conflicting"
)

// SplitStats describes one paired measurement of candidate against baseline.
type SplitStats struct {
	// Split is the split label.
	Split string
	// N is the number of items both systems were measured on.
	N int
	// Items are the shared item names, sorted.
	Items []string
	// Mean is the mean paired difference, candidate minus baseline.
	Mean float64
	// StdErr is the standard error of that mean.
	StdErr float64
	// StdDev is the sample standard deviation of the paired differences.
	StdDev float64
}

// Result is the outcome of one gate evaluation.
type Result struct {
	// Manifest is the contract the evaluation was run under.
	Manifest *manifest.Manifest
	// Selection holds the statistics of the selection split.
	Selection SplitStats
	// Heldout holds the statistics of the held-out split.
	Heldout SplitStats

	// EffectiveCandidates is the number of candidates that behave independently.
	EffectiveCandidates int
	// ExpectedMax is the expected maximum of that many standard normals.
	ExpectedMax float64
	// Inflation is the expected selection inflation in absolute gain.
	Inflation float64
	// ClaimedGain is the gain reported on the selection split.
	ClaimedGain float64
	// CorrectedClaim is the claimed gain with the expected selection
	// inflation removed.
	CorrectedClaim float64

	// Z is the two-sided critical value used for the interval.
	Z float64
	// HalfWidth is the half width of the held-out confidence interval.
	HalfWidth float64
	// HeldoutGain is the mean difference on the held-out split.
	HeldoutGain float64
	// LowerBound is the lower end of the held-out confidence interval.
	LowerBound float64
	// UpperBound is the upper end of the held-out confidence interval.
	UpperBound float64

	// Verdict is one of VerdictProven, VerdictNotProven, VerdictConflicting.
	Verdict string
	// Reason explains the verdict in one sentence.
	Reason string

	// RequiredHeldout is how many held-out items the declared minimum gain
	// needs at the declared significance level and power.
	RequiredHeldout int
	// SigmaItem is the per-item standard deviation used for that planning.
	SigmaItem float64
	// SigmaSource records how SigmaItem was obtained.
	SigmaSource string
	// SigmaSourceKey is the translation key of SigmaSource.
	SigmaSourceKey string
}

// Evaluate runs the gate over a dataset.
func Evaluate(m *manifest.Manifest, d *scores.Dataset) (*Result, error) {
	if m == nil {
		return nil, fmt.Errorf("no manifest")
	}
	if d == nil {
		return nil, fmt.Errorf("no data")
	}

	selDiffs, selItems, err := d.Pair(m.Selection, m.Baseline, m.Candidate)
	if err != nil {
		return nil, fmt.Errorf("selection split: %w", err)
	}
	if len(selDiffs) < 2 {
		return nil, fmt.Errorf("selection split %q has %d paired items, need at least 2", m.Selection, len(selDiffs))
	}
	holdDiffs, holdItems, err := d.Pair(m.Heldout, m.Baseline, m.Candidate)
	if err != nil {
		return nil, fmt.Errorf("held-out split: %w", err)
	}
	if len(holdDiffs) < 2 {
		return nil, fmt.Errorf("held-out split %q has %d paired items, need at least 2", m.Heldout, len(holdDiffs))
	}

	res := &Result{Manifest: m}

	res.Selection = splitStats(m.Selection, selDiffs, selItems)
	res.Heldout = splitStats(m.Heldout, holdDiffs, holdItems)

	res.EffectiveCandidates = stats.EffectiveCandidates(m.Candidates, m.Corr)
	res.ExpectedMax = stats.ExpectedMaxNormal(m.Candidates, m.Corr)
	res.Inflation = res.Selection.StdErr * res.ExpectedMax
	res.ClaimedGain = res.Selection.Mean
	res.CorrectedClaim = res.ClaimedGain - res.Inflation

	res.Z = stats.TwoSidedZ(m.Alpha)
	res.HalfWidth = res.Z * res.Heldout.StdErr
	res.HeldoutGain = res.Heldout.Mean
	res.LowerBound = res.HeldoutGain - res.HalfWidth
	res.UpperBound = res.HeldoutGain + res.HalfWidth

	switch {
	case res.LowerBound > m.MinGain:
		res.Verdict = VerdictProven
		res.Reason = fmt.Sprintf(
			"the held-out lower bound %.4f clears the minimum gain %.4f",
			res.LowerBound, m.MinGain)
	case res.ClaimedGain > 0:
		res.Verdict = VerdictConflicting
		res.Reason = fmt.Sprintf(
			"the selection split reports %+.4f but the held-out lower bound %.4f does not clear %.4f",
			res.ClaimedGain, res.LowerBound, m.MinGain)
	default:
		res.Verdict = VerdictNotProven
		res.Reason = fmt.Sprintf(
			"the held-out lower bound %.4f does not clear the minimum gain %.4f",
			res.LowerBound, m.MinGain)
	}

	res.SigmaItem, res.SigmaSource = estimateSigma(holdDiffs)
	res.SigmaSourceKey = "sigma_worst"
	if res.SigmaSource == sourceEstimated {
		res.SigmaSourceKey = "sigma_estimated"
	}
	if n, ok := stats.RequiredPairedN(m.MinGain, m.Alpha, m.Power, res.SigmaItem); ok {
		res.RequiredHeldout = n
	}
	return res, nil
}

// splitStats summarises one paired measurement.
func splitStats(split string, diffs []float64, items []string) SplitStats {
	mean, _ := stats.Mean(diffs)
	sd, _ := stats.StdDev(diffs)
	se, _ := stats.StdError(diffs)
	return SplitStats{
		Split:   split,
		N:       len(diffs),
		Items:   items,
		Mean:    mean,
		StdDev:  sd,
		StdErr:  se,
	}
}

// sourceEstimated names the origin of a spread estimated from data.
const sourceEstimated = "estimated from the held-out split"

// sourceWorstCase names the conservative spread used when no data is available.
const sourceWorstCase = "worst case for a binary metric"

// estimateSigma returns the per-item standard deviation of the paired
// difference and where it came from.
//
// When the held-out split already carries enough paired items, the value is
// estimated from it. Otherwise the worst case for a binary metric is used,
// which is the conservative choice: it never understates the sample size.
func estimateSigma(holdDiffs []float64) (float64, string) {
	if len(holdDiffs) >= 2 {
		sd, ok := stats.StdDev(holdDiffs)
		if ok && sd > 0 {
			return sd, sourceEstimated
		}
	}
	return stats.MaxPairedStdDev(), sourceWorstCase
}

// Plan describes how much held-out evidence a declared gain needs.
type Plan struct {
	// MinGain is the smallest true gain worth detecting.
	MinGain float64
	// Alpha is the two-sided significance level.
	Alpha float64
	// Power is the target probability of detecting a real gain.
	Power float64
	// SigmaItem is the per-item standard deviation of the paired difference.
	SigmaItem float64
	// SigmaSource records how SigmaItem was obtained.
	SigmaSource string
	// RequiredHeldout is the number of paired items needed.
	RequiredHeldout int
	// SelectionItems is the size of the selection split, when known.
	SelectionItems int
	// Candidates is how many candidates the round produced.
	Candidates int
	// ExpectedMax is the expected maximum of that many standard normals.
	ExpectedMax float64
	// ExpectedInflation is the expected overstatement of the selection-split
	// gain at the given selection size.
	ExpectedInflation float64
}

// MakePlan builds a plan for a declared contract.
//
// When heldDiffs is not empty the per-item standard deviation is estimated
// from it; otherwise the worst case for a binary metric is used.
func MakePlan(m *manifest.Manifest, selectionItems int, heldDiffs []float64) Plan {
	sigma, source := estimateSigma(heldDiffs)
	p := Plan{
		MinGain:        m.MinGain,
		Alpha:          m.Alpha,
		Power:          m.Power,
		SigmaItem:      sigma,
		SigmaSource:    source,
		SelectionItems: selectionItems,
		Candidates:     m.Candidates,
		ExpectedMax:    stats.ExpectedMaxNormal(m.Candidates, m.Corr),
	}
	if n, ok := stats.RequiredPairedN(m.MinGain, m.Alpha, m.Power, sigma); ok {
		p.RequiredHeldout = n
	}
	if selectionItems >= 2 {
		p.ExpectedInflation = StandardErrorFor(selectionItems, sigma) * p.ExpectedMax
	}
	return p
}

// StandardErrorFor returns the standard error of the mean of n paired
// differences with the given per-item standard deviation.
func StandardErrorFor(n int, sigma float64) float64 {
	if n < 2 {
		return 0
	}
	return sigma / math.Sqrt(float64(n))
}
