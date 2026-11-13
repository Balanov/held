package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Balanov/held/gate"
	"github.com/Balanov/held/manifest"
	"github.com/Balanov/held/stats"
)

// ruleResult is the measured behaviour of one acceptance rule.
type ruleResult struct {
	// Name identifies the rule.
	Name string `json:"name"`
	// AcceptRate is the share of rounds in which the rule accepted.
	AcceptRate float64 `json:"accept_rate"`
	// FalseAcceptRate is the share of accepted rounds in the null scenario.
	FalseAcceptRate float64 `json:"false_accept_rate,omitempty"`
	// FalseRejectRate is the share of rejected rounds in the alternative scenario.
	FalseRejectRate float64 `json:"false_reject_rate,omitempty"`
}

// configResult is the measured behaviour of one configuration.
type configResult struct {
	SelectionItems int `json:"selection_items"`
	HeldoutItems   int `json:"heldout_items"`
	Candidates     int `json:"candidates"`
	MinGain        float64 `json:"min_gain"`
	MeanClaimedGain float64 `json:"mean_claimed_gain"`
	MeanHeldoutGain float64 `json:"mean_heldout_gain"`
	ObservedOverstatement float64 `json:"observed_overstatement"`
	PredictedInflation    float64 `json:"predicted_inflation"`
	Rules []ruleResult `json:"rules"`
}

// scenarioResult groups the configurations of one scenario.
type scenarioResult struct {
	Name      string         `json:"name"`
	TrueDelta float64        `json:"true_delta"`
	Rounds    int            `json:"rounds"`
	Configs   []configResult `json:"configurations"`
}

// crossCheck reports whether the shipped gate agrees with the harness.
type crossCheck struct {
	Rounds     int `json:"rounds"`
	Mismatches int `json:"mismatches"`
	Proven     int `json:"proven"`
	Conflicting int `json:"conflicting"`
	NotProven  int `json:"not_proven"`
}

// report is the whole benchmark output.
type report struct {
	Tool        string          `json:"tool"`
	Version     string          `json:"version"`
	Generator   string          `json:"generator"`
	Alpha       float64         `json:"alpha"`
	Power       float64         `json:"power"`
	CrossCheck  crossCheck      `json:"cross_check"`
	Scenarios   []scenarioResult `json:"scenarios"`
	Notes       []string        `json:"notes"`
}

func main() {
	outDir := flag.String("out", "bench/results", "directory for the result files")
	rounds := flag.Int("rounds", 2000, "rounds per configuration")
	flag.Parse()

	if err := run(*outDir, *rounds); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

// run executes the whole benchmark and writes its output.
func run(outDir string, rounds int) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	alpha := 0.05
	power := 0.8
	heldoutItems := 1000

	rep := report{
		Tool:      "held",
		Version:   "0.1.0",
		Generator: "splitmix64",
		Alpha:     alpha,
		Power:     power,
		Notes: []string{
			"Every acceptance rule sees the same rounds; the rules differ only in the data they read.",
			"The null scenario has no real improvement, so every acceptance is a false accept.",
			"The alternative scenario has a real improvement of 0.05, so every rejection is a false reject.",
			"The held-out set is drawn fresh in each round, so the reported rate is the per-round rate of a rule that is applied once to one round.",
			"The selection set is shared by all candidates in a round, and the candidate with the best selection score is the one that is judged.",
		},
	}

	cc, err := crossCheckTheGate(rounds/10 + 1)
	if err != nil {
		return err
	}
	rep.CrossCheck = cc

	selectionSizes := []int{16, 64, 256}
	candidateCounts := []int{1, 4, 16, 64}
	minGains := []float64{0, 0.05}

	for _, scenario := range []struct {
		name      string
		trueDelta float64
	}{
		{"null", 0},
		{"alternative", 0.05},
	} {
		sc := scenarioResult{Name: scenario.name, TrueDelta: scenario.trueDelta, Rounds: rounds}
		seed := uint64(0x5EED0000) ^ uint64(len(scenario.name))
		for _, sel := range selectionSizes {
			for _, cands := range candidateCounts {
				for _, mg := range minGains {
					cfg := config{
						SelectionItems: sel,
						HeldoutItems:   heldoutItems,
						Candidates:     cands,
						TrueDelta:      scenario.trueDelta,
						MinGain:        mg,
						Alpha:          alpha,
						Rounds:         rounds,
						Seed:           seed,
					}
					seed++

					roundsOut, err := simulate(cfg)
					if err != nil {
						return err
					}
					cr, err := summarise(cfg, roundsOut, scenario.name)
					if err != nil {
						return err
					}
					sc.Configs = append(sc.Configs, cr)
				}
			}
		}
		rep.Scenarios = append(rep.Scenarios, sc)
	}

	encoded, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	jsonPath := filepath.Join(outDir, "benchmark.json")
	if err := os.WriteFile(jsonPath, encoded, 0o644); err != nil {
		return err
	}
	summary := renderSummary(rep)
	if err := os.WriteFile(filepath.Join(outDir, "summary.md"), []byte(summary), 0o644); err != nil {
		return err
	}
	fmt.Print(summary)
	fmt.Println("wrote " + jsonPath)
	return nil
}

// summarise reduces the rounds of one configuration to measured rates.
func summarise(cfg config, rounds []round, scenario string) (configResult, error) {
	cr := configResult{
		SelectionItems: cfg.SelectionItems,
		HeldoutItems:   cfg.HeldoutItems,
		Candidates:     cfg.Candidates,
		MinGain:        cfg.MinGain,
	}

	accepts := make([]int, len(ruleOrder))
	var claimed, heldout, seSum float64
	for _, rd := range rounds {
		claimed += rd.WinnerSelectionGain
		heldout += rd.WinnerHeldoutGain
		seSum += rd.SelectionStdErr
		for i, ok := range rd.Accepted {
			if ok {
				accepts[i]++
			}
		}
	}
	n := float64(len(rounds))
	cr.MeanClaimedGain = claimed / n
	cr.MeanHeldoutGain = heldout / n
	cr.ObservedOverstatement = cr.MeanClaimedGain - cfg.TrueDelta
	cr.PredictedInflation = (seSum / n) * stats.ExpectedMaxNormal(cfg.Candidates, 0)

	for i, name := range ruleOrder {
		rate := float64(accepts[i]) / n
		rr := ruleResult{Name: name, AcceptRate: rate}
		if scenario == "null" {
			rr.FalseAcceptRate = rate
		} else {
			rr.FalseRejectRate = 1 - rate
		}
		cr.Rules = append(cr.Rules, rr)
	}
	return cr, nil
}

// crossCheckTheGate runs the shipped gate over simulated rounds and compares
// its verdict with the harness decision.
func crossCheckTheGate(rounds int) (crossCheck, error) {
	cfg := config{
		SelectionItems: 32,
		HeldoutItems:   256,
		Candidates:     8,
		TrueDelta:      0.02,
		MinGain:        0,
		Alpha:          0.05,
		Rounds:         rounds,
		Seed:           0xC0FFEE,
		KeepRaw:        true,
	}
	out, err := simulate(cfg)
	if err != nil {
		return crossCheck{}, err
	}
	m, err := manifest.Parse("v=1, candidate=cand, baseline=base, metric=success_rate, " +
		"judge=state_check, selection=sel, heldout=hold, candidates=8, alpha=0.05, min_gain=0")
	if err != nil {
		return crossCheck{}, err
	}

	cc := crossCheck{Rounds: len(out)}
	for _, rd := range out {
		if rd.draws == nil {
			continue
		}
		d, err := buildDataset(cfg, *rd.draws)
		if err != nil {
			return cc, err
		}
		res, err := gate.Evaluate(m, d)
		if err != nil {
			return cc, err
		}
		switch res.Verdict {
		case gate.VerdictProven:
			cc.Proven++
		case gate.VerdictConflicting:
			cc.Conflicting++
		default:
			cc.NotProven++
		}
		// The harness marks the held-out rule as accepting when the lower
		// bound clears the minimum gain; the shipped gate must agree.
		harnessAccepted := rd.Accepted[2]
		gateAccepted := res.Verdict == gate.VerdictProven
		if harnessAccepted != gateAccepted {
			cc.Mismatches++
		}
	}
	return cc, nil
}

// renderSummary builds the human-readable table.
func renderSummary(rep report) string {
	var b strings.Builder
	b.WriteString("# HELD benchmark\n\n")
	fmt.Fprintf(&b, "Alpha %.2f, power %.2f, generator %s, cross-check mismatches %d of %d rounds.\n\n",
		rep.Alpha, rep.Power, rep.Generator, rep.CrossCheck.Mismatches, rep.CrossCheck.Rounds)

	for _, sc := range rep.Scenarios {
		fmt.Fprintf(&b, "## Scenario: %s (true gain %.2f)\n\n", sc.Name, sc.TrueDelta)
		b.WriteString("| selection | held-out | candidates | min gain | rule | accept rate | ")
		if sc.Name == "null" {
			b.WriteString("false accept |")
		} else {
			b.WriteString("false reject |")
		}
		b.WriteString(" claimed gain | overstatement | predicted |\n")
		b.WriteString("|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|\n")
		for _, cr := range sc.Configs {
			for _, r := range cr.Rules {
				rate := r.FalseAcceptRate
				if sc.Name != "null" {
					rate = r.FalseRejectRate
				}
				fmt.Fprintf(&b, "| %d | %d | %d | %.2f | %s | %.4f | %.4f | %+.4f | %+.4f | %+.4f |\n",
					cr.SelectionItems, cr.HeldoutItems, cr.Candidates, cr.MinGain, r.Name,
					r.AcceptRate, rate, cr.MeanClaimedGain, cr.ObservedOverstatement, cr.PredictedInflation)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Notes\n\n")
	for _, n := range rep.Notes {
		b.WriteString("- " + n + "\n")
	}
	return b.String()
}
