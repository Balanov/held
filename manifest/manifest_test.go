package manifest

import (
	"strings"
	"testing"
)

const good = "v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate, " +
	"judge=state_check, selection=sel, heldout=hold, candidates=16, alpha=0.05, " +
	"min_gain=0.01, power=0.8, locale=en;ru;es;zh"

func TestParseGood(t *testing.T) {
	m, err := Parse(good)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Version != 1 {
		t.Errorf("Version = %d", m.Version)
	}
	if m.Candidate != "policy-v7" || m.Baseline != "policy-v3" {
		t.Errorf("candidate/baseline = %q/%q", m.Candidate, m.Baseline)
	}
	if m.Metric != "success_rate" {
		t.Errorf("Metric = %q", m.Metric)
	}
	if m.Judge != "state_check" {
		t.Errorf("Judge = %q", m.Judge)
	}
	if m.Selection != "sel" || m.Heldout != "hold" {
		t.Errorf("splits = %q/%q", m.Selection, m.Heldout)
	}
	if m.Candidates != 16 {
		t.Errorf("Candidates = %d", m.Candidates)
	}
	if m.Alpha != 0.05 || m.MinGain != 0.01 || m.Power != 0.8 {
		t.Errorf("thresholds = %v %v %v", m.Alpha, m.MinGain, m.Power)
	}
	if len(m.Locales) != 4 {
		t.Errorf("Locales = %v", m.Locales)
	}
}

func TestParseDefaults(t *testing.T) {
	m, err := Parse("v=1, candidate=c, baseline=b, metric=m, selection=sel, heldout=hold")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Judge != DefaultJudge {
		t.Errorf("Judge default = %q", m.Judge)
	}
	if m.Candidates != DefaultCandidates {
		t.Errorf("Candidates default = %d", m.Candidates)
	}
	if m.Alpha != DefaultAlpha {
		t.Errorf("Alpha default = %v", m.Alpha)
	}
	if m.Power != DefaultPower {
		t.Errorf("Power default = %v", m.Power)
	}
	if len(m.Locales) != 1 || m.Locales[0] != DefaultLocale {
		t.Errorf("Locales default = %v", m.Locales)
	}
}

func TestCanonicalIsStableAcrossFormatting(t *testing.T) {
	wrapped := "v=1, candidate=policy-v7,\n  baseline=policy-v3, metric=success_rate,\n" +
		"  judge=state_check, selection=sel, heldout=hold, candidates=16,\n" +
		"  alpha=0.05, min_gain=0.01, power=0.8, locale=en;ru;es;zh\n"
	withComment := good + "\n# a note about the contract\n"

	a, err := Parse(good)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(withComment)
	if err != nil {
		t.Fatal(err)
	}
	if a.Canonical() != b.Canonical() || a.Canonical() != c.Canonical() {
		t.Errorf("canonical forms differ:\n%s\n%s\n%s", a.Canonical(), b.Canonical(), c.Canonical())
	}
	if a.Hash() != b.Hash() || a.Hash() != c.Hash() {
		t.Error("pins differ across formatting")
	}
}

func TestCanonicalOrdersClauses(t *testing.T) {
	m, err := Parse("heldout=hold, baseline=b, v=1, metric=m, candidate=c, selection=sel")
	if err != nil {
		t.Fatal(err)
	}
	want := "v=1, candidate=c, baseline=b, metric=m, judge=state_check, selection=sel, heldout=hold, " +
		"candidates=1, corr=0, alpha=0.05, min_gain=0, power=0.8, locale=en"
	if m.Canonical() != want {
		t.Errorf("canonical = %q\nwant %q", m.Canonical(), want)
	}
}

func TestHashChangesWithContent(t *testing.T) {
	a, _ := Parse(good)
	b, _ := Parse(strings.Replace(good, "min_gain=0.01", "min_gain=0.02", 1))
	if a.Hash() == b.Hash() {
		t.Error("the pin did not change when a clause changed")
	}
}

func TestHashIsStable(t *testing.T) {
	a, _ := Parse(good)
	first := a.Hash()
	for i := 0; i < 5; i++ {
		if a.Hash() != first {
			t.Fatal("the pin is not stable")
		}
	}
	if len(first) != 64 {
		t.Errorf("pin length = %d", len(first))
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"unknown clause", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, wat=1"},
		{"duplicate clause", "v=1, candidate=c, candidate=d, baseline=b, metric=m, selection=s, heldout=h"},
		{"missing candidate", "v=1, baseline=b, metric=m, selection=s, heldout=h"},
		{"missing heldout", "v=1, candidate=c, baseline=b, metric=m, selection=s"},
		{"bad version", "v=2, candidate=c, baseline=b, metric=m, selection=s, heldout=h"},
		{"bad alpha", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, alpha=1.5"},
		{"bad power", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, power=0"},
		{"bad corr", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, corr=2"},
		{"zero candidates", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, candidates=0"},
		{"same split", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=s"},
		{"unknown locale", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, locale=xx"},
		{"not a number", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, alpha=abc"},
		{"not an integer", "v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, candidates=many"},
		{"empty value", "v=1, candidate=, baseline=b, metric=m, selection=s, heldout=h"},
	}
	for _, c := range cases {
		if _, err := Parse(c.text); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}
}

func TestParseQuotedValueWithComma(t *testing.T) {
	m, err := Parse("v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, " +
		`note="round 12, after the retune"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Note != "round 12, after the retune" {
		t.Errorf("Note = %q", m.Note)
	}
	if !strings.Contains(m.Canonical(), "note=\"round 12, after the retune\"") {
		t.Errorf("canonical = %q", m.Canonical())
	}
	// The canonical form must parse back to the same thing.
	again, err := Parse(m.Canonical())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if again.Note != m.Note || again.Hash() != m.Hash() {
		t.Error("the canonical form did not round trip")
	}
}

func TestParseUnterminatedQuote(t *testing.T) {
	if _, err := Parse("v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, note=\"open"); err == nil {
		t.Error("expected an error for an unterminated quote")
	}
}

func TestParseLocaleList(t *testing.T) {
	m, err := Parse("v=1, candidate=c, baseline=b, metric=m, selection=s, heldout=h, locale=en;ru")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Locales) != 2 || m.Locales[0] != "en" || m.Locales[1] != "ru" {
		t.Errorf("Locales = %v", m.Locales)
	}
}
