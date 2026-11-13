// Package report renders a measurement contract and its verdict as text.
//
// The output is a protocol document: the declared contract, the verdict, and
// the numbers behind it. It is written to be pasted into a paper, a review or
// a safety case, which is why the labels are aligned and the numbers carry
// explicit signs.
package report

import (
	"fmt"
	"strings"

	"github.com/Balanov/held/gate"
	"github.com/Balanov/held/i18n"
	"github.com/Balanov/held/manifest"
)

// signed formats a number with an explicit sign.
func signed(v float64) string {
	return fmt.Sprintf("%+.4f", v)
}

// plain formats a number without a sign.
func plain(v float64) string {
	return fmt.Sprintf("%.4f", v)
}

// pair renders a label and a value in a fixed-width field. Extra fragments are
// appended in brackets as context.
func pair(label string, value any, extra ...string) string {
	line := fmt.Sprintf("%-30s %v", label, value)
	if len(extra) > 0 {
		line += "  (" + strings.Join(extra, ", ") + ")"
	}
	return line
}

// Protocol renders the full protocol document in one language.
func Protocol(m *manifest.Manifest, res *gate.Result, locale string) string {
	var b strings.Builder
	t := func(key string) string { return i18n.T(locale, key) }
	line := func(label string, value any, extra ...string) {
		fmt.Fprintf(&b, "%s\n", pair(label, value, extra...))
	}

	fmt.Fprintf(&b, "HELD · %s\n", t("title"))
	fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 64))
	line(t("contract"), m.Canonical())
	line(t("pin"), m.Hash())
	b.WriteString("\n")

	line(t("candidate"), m.Candidate)
	line(t("baseline"), m.Baseline)
	line(t("metric"), m.Metric)
	line(t("judge"), m.Judge)
	if res != nil {
		line(t("selection"), fmt.Sprintf("%s  %d %s  %s",
			m.Selection, res.Selection.N, t("items"), t("selected_on")))
		line(t("heldout"), fmt.Sprintf("%s  %d %s  %s",
			m.Heldout, res.Heldout.N, t("items"), t("never_used")))
	} else {
		line(t("selection"), m.Selection)
		line(t("heldout"), m.Heldout)
	}
	line(t("candidates"), m.Candidates)
	line(t("alpha"), plain(m.Alpha))
	line(t("min_gain"), plain(m.MinGain))
	line(t("power"), plain(m.Power))
	if m.Note != "" {
		line(t("note"), m.Note)
	}
	b.WriteString("\n")

	if res != nil {
		line(t("verdict"), t("verdict_"+res.Verdict))
		if reason := i18n.Reason(locale, res.Verdict, reasonArgs(res, m)...); reason != "" {
			fmt.Fprintf(&b, "%-22s %s\n", "", reason)
		}
		b.WriteString("\n")

		line(t("claimed_gain"), signed(res.ClaimedGain))
		line(t("selection_inflation"), plain(res.Inflation),
			fmt.Sprintf("%d %s", res.EffectiveCandidates, t("candidates")),
			fmt.Sprintf("%s %.4f", t("expected_max"), res.ExpectedMax))
		line(t("corrected_claim"), signed(res.CorrectedClaim))
		line(t("heldout_gain"), signed(res.HeldoutGain))
		line(t("interval"), fmt.Sprintf("[%s, %s]", signed(res.LowerBound), signed(res.UpperBound)))
		line(t("lower_bound"), signed(res.LowerBound))
		line(t("required_heldout"), res.RequiredHeldout)
		line(t("sigma"), plain(res.SigmaItem), t(res.SigmaSourceKey))
	}
	return b.String()
}

// reasonArgs supplies the values substituted into the verdict explanation.
func reasonArgs(res *gate.Result, m *manifest.Manifest) []string {
	switch res.Verdict {
	case gate.VerdictConflicting:
		return []string{signed(res.ClaimedGain), signed(res.LowerBound), plain(m.MinGain)}
	case gate.VerdictProven:
		return []string{signed(res.LowerBound), plain(m.MinGain)}
	default:
		return []string{signed(res.LowerBound), plain(m.MinGain)}
	}
}

// Summary renders a single-line verdict for a terminal or a CI log.
func Summary(res *gate.Result, m *manifest.Manifest, locale string) string {
	t := func(key string) string { return i18n.T(locale, key) }
	return fmt.Sprintf("%s: %s (claimed %s, held-out %s, lower bound %s)",
		t("verdict"), t("verdict_"+res.Verdict),
		signed(res.ClaimedGain), signed(res.HeldoutGain), signed(res.LowerBound))
}
