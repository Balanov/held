// Package manifest reads and writes the HELD measurement contract.
//
// The contract is one line of clauses separated by commas:
//
//	v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate,
//	selection=sel, heldout=hold, candidates=16, alpha=0.05, min_gain=0.01,
//	locale=en;ru;es;zh
//
// The comma is the whole grammar. A clause is key=value; the value may be
// quoted, and a quoted value may contain commas. A semicolon separates the
// elements of a list-valued clause such as locale. A hash starts a comment
// that runs to the end of the line. Newlines are ordinary whitespace, so a
// long contract may be wrapped after any comma and still parse to exactly the
// same thing.
//
// The canonical form is a single line with a fixed clause order and normalised
// values. Its SHA-256 is the pin of the contract: the measurement protocol is
// hashed, so a claim cannot be re-described after the fact without breaking
// the pin.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is the current contract format version.
const Version = 1

// Manifest is a parsed measurement contract.
type Manifest struct {
	// Version is the contract format version.
	Version int
	// Candidate identifies the system under test.
	Candidate string
	// Baseline identifies the system the candidate is compared against.
	Baseline string
	// Metric names the measured quantity, for example success_rate.
	Metric string
	// Judge names how an outcome was decided, for example state_check.
	Judge string
	// Selection is the split label of the data used to pick the candidate.
	Selection string
	// Heldout is the split label of the data never used to pick it.
	Heldout string
	// Candidates is how many candidates the round produced before selection.
	Candidates int
	// Corr is the assumed correlation between candidates in one round.
	Corr float64
	// Alpha is the two-sided significance level.
	Alpha float64
	// MinGain is the smallest true gain worth accepting.
	MinGain float64
	// Power is the target power used when planning a sample size.
	Power float64
	// Locales lists the languages the contract is rendered into.
	Locales []string
	// Note is a free-text annotation carried into every rendering.
	Note string
}

// Problem is a single validation failure.
type Problem struct {
	// Clause is the offending clause text, empty for whole-document problems.
	Clause string
	// Key is the offending clause key.
	Key string
	// Message explains the failure.
	Message string
}

func (p Problem) String() string {
	if p.Clause != "" {
		return fmt.Sprintf("%s: %s", p.Key, p.Message)
	}
	return p.Message
}

// Error reports one or more validation problems.
type Error struct {
	Problems []Problem
}

func (e *Error) Error() string {
	if len(e.Problems) == 1 {
		return "invalid manifest: " + e.Problems[0].String()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "invalid manifest: %d problems", len(e.Problems))
	for _, p := range e.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p.String())
	}
	return b.String()
}

// Defaults for the optional clauses.
const (
	DefaultJudge       = "state_check"
	DefaultCandidates  = 1
	DefaultAlpha       = 0.05
	DefaultMinGain     = 0.0
	DefaultPower       = 0.8
	DefaultCorr        = 0.0
	DefaultLocale      = "en"
)

// clauseOrder is the canonical order of clauses. It is fixed so that the
// canonical form, and therefore the pin, does not depend on how the contract
// happened to be written.
var clauseOrder = []string{
	"v", "candidate", "baseline", "metric", "judge",
	"selection", "heldout", "candidates", "corr",
	"alpha", "min_gain", "power", "locale", "note",
}

// knownLocales are the languages the renderer ships with.
var knownLocales = map[string]bool{
	"en": true, "ru": true, "es": true, "zh": true,
}

// clause holds one raw key/value pair before validation.
type clause struct {
	key   string
	value string
	quote bool
}

// Parse reads a contract from text.
//
// Parsing is fail-closed: an unknown key, a repeated key, a missing required
// clause, a malformed value or an unsupported version is an error, never a
// silent default.
func Parse(text string) (*Manifest, error) {
	raw, err := splitClauses(text)
	if err != nil {
		return nil, &Error{Problems: []Problem{{Message: err.Error()}}}
	}

	seen := map[string]clause{}
	var problems []Problem
	for _, c := range raw {
		if c.key == "" {
			problems = append(problems, Problem{Clause: c.value, Message: "clause has no key"})
			continue
		}
		if _, dup := seen[c.key]; dup {
			problems = append(problems, Problem{Clause: c.key, Key: c.key, Message: "clause appears more than once"})
			continue
		}
		seen[c.key] = c
	}

	m := &Manifest{
		Judge:      DefaultJudge,
		Candidates: DefaultCandidates,
		Alpha:      DefaultAlpha,
		MinGain:    DefaultMinGain,
		Power:      DefaultPower,
		Corr:       DefaultCorr,
	}

	for _, key := range []string{"v", "candidate", "baseline", "metric", "selection", "heldout"} {
		if _, ok := seen[key]; !ok {
			problems = append(problems, Problem{Key: key, Message: "required clause is missing"})
		}
	}

	readString := func(key string, dst *string) {
		c, ok := seen[key]
		if !ok {
			return
		}
		if c.value == "" {
			problems = append(problems, Problem{Clause: key, Key: key, Message: "clause value is empty"})
			return
		}
		*dst = c.value
	}
	readInt := func(key string, dst *int) {
		c, ok := seen[key]
		if !ok {
			return
		}
		n, err := strconv.Atoi(strings.TrimSpace(c.value))
		if err != nil {
			problems = append(problems, Problem{Clause: key, Key: key, Message: "clause value is not an integer"})
			return
		}
		*dst = n
	}
	readFloat := func(key string, dst *float64) {
		c, ok := seen[key]
		if !ok {
			return
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(c.value), 64)
		if err != nil {
			problems = append(problems, Problem{Clause: key, Key: key, Message: "clause value is not a number"})
			return
		}
		*dst = f
	}

	readInt("v", &m.Version)
	readString("candidate", &m.Candidate)
	readString("baseline", &m.Baseline)
	readString("metric", &m.Metric)
	readString("judge", &m.Judge)
	readString("selection", &m.Selection)
	readString("heldout", &m.Heldout)
	readInt("candidates", &m.Candidates)
	readFloat("corr", &m.Corr)
	readFloat("alpha", &m.Alpha)
	readFloat("min_gain", &m.MinGain)
	readFloat("power", &m.Power)
	readString("note", &m.Note)

	if c, ok := seen["locale"]; ok {
		locales := splitList(c.value)
		if len(locales) == 0 {
			problems = append(problems, Problem{Clause: "locale", Key: "locale", Message: "clause value is empty"})
		}
		for _, l := range locales {
			if !knownLocales[l] {
				problems = append(problems, Problem{
					Clause: "locale=" + l, Key: "locale",
					Message: "unknown locale " + l + " (known: en, ru, es, zh)",
				})
			}
		}
		m.Locales = locales
	} else {
		m.Locales = []string{DefaultLocale}
	}

	for key := range seen {
		if !knownKey(key) {
			problems = append(problems, Problem{
				Clause: key, Key: key,
				Message: "unknown clause (known: " + strings.Join(clauseOrder, ", ") + ")",
			})
		}
	}

	if m.Version != Version {
		problems = append(problems, Problem{
			Clause: fmt.Sprintf("v=%d", m.Version), Key: "v",
			Message: fmt.Sprintf("unsupported contract version %d (this build supports %d)", m.Version, Version),
		})
	}
	if m.Candidates < 1 {
		problems = append(problems, Problem{
			Clause: fmt.Sprintf("candidates=%d", m.Candidates), Key: "candidates",
			Message: "candidates must be at least 1",
		})
	}
	if m.Corr < 0 || m.Corr > 1 {
		problems = append(problems, Problem{
			Clause: fmt.Sprintf("corr=%v", m.Corr), Key: "corr",
			Message: "corr must be between 0 and 1",
		})
	}
	if m.Alpha <= 0 || m.Alpha >= 1 {
		problems = append(problems, Problem{
			Clause: fmt.Sprintf("alpha=%v", m.Alpha), Key: "alpha",
			Message: "alpha must be strictly between 0 and 1",
		})
	}
	if m.Power <= 0 || m.Power >= 1 {
		problems = append(problems, Problem{
			Clause: fmt.Sprintf("power=%v", m.Power), Key: "power",
			Message: "power must be strictly between 0 and 1",
		})
	}
	if m.Selection == m.Heldout {
		problems = append(problems, Problem{
			Clause: "heldout=" + m.Heldout, Key: "heldout",
			Message: "held-out split must differ from the selection split",
		})
	}

	if len(problems) > 0 {
		sort.SliceStable(problems, func(i, j int) bool { return problems[i].Key < problems[j].Key })
		return nil, &Error{Problems: problems}
	}
	return m, nil
}

func knownKey(key string) bool {
	for _, k := range clauseOrder {
		if k == key {
			return true
		}
	}
	return false
}

// splitClauses tokenises the contract text into raw clauses.
func splitClauses(text string) ([]clause, error) {
	var out []clause
	var b strings.Builder
	inQuote := false
	escaped := false
	started := false

	flush := func() {
		part := strings.TrimSpace(b.String())
		b.Reset()
		if part == "" {
			return
		}
		key, value, quoted, err := splitKeyValue(part)
		if err != nil {
			out = append(out, clause{key: "", value: part})
			return
		}
		out = append(out, clause{key: key, value: value, quote: quoted})
	}

	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inQuote {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"':
			inQuote = true
			started = true
			b.WriteByte(ch)
		case '#':
			// Comment to the end of the line.
			for i < len(text) && text[i] != '\n' {
				i++
			}
			if i < len(text) {
				i--
			}
		case ',':
			flush()
			started = false
		case '\n', '\r':
			// A newline may follow a comma or separate clauses; treat it as
			// whitespace inside the clause.
			if strings.TrimSpace(b.String()) != "" {
				b.WriteByte(' ')
			}
		default:
			started = true
			b.WriteByte(ch)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted value")
	}
	flush()
	_ = started
	return out, nil
}

// splitKeyValue splits one clause at its first unquoted equals sign.
func splitKeyValue(part string) (key, value string, quoted bool, err error) {
	inQuote := false
	escaped := false
	for i := 0; i < len(part); i++ {
		ch := part[i]
		if inQuote {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"':
			inQuote = true
		case '=':
			key = strings.TrimSpace(part[:i])
			raw := strings.TrimSpace(part[i+1:])
			if strings.HasPrefix(raw, "\"") {
				unquoted, uerr := unquote(raw)
				if uerr != nil {
					return "", "", false, uerr
				}
				return key, unquoted, true, nil
			}
			return key, raw, false, nil
		}
	}
	return "", part, false, fmt.Errorf("clause has no '='")
}

// unquote removes the surrounding quotes from a quoted value.
func unquote(raw string) (string, error) {
	if len(raw) < 2 || raw[0] != '"' {
		return "", fmt.Errorf("malformed quoted value")
	}
	var b strings.Builder
	escaped := false
	closed := false
	for i := 1; i < len(raw); i++ {
		ch := raw[i]
		if escaped {
			switch ch {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				b.WriteByte('\\')
				b.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			closed = true
			if strings.TrimSpace(raw[i+1:]) != "" {
				return "", fmt.Errorf("trailing characters after quoted value")
			}
			break
		}
		b.WriteByte(ch)
	}
	if !closed {
		return "", fmt.Errorf("unterminated quoted value")
	}
	return b.String(), nil
}

// splitList splits a list-valued clause on semicolons.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// Canonical renders the contract as a single line with a fixed clause order.
//
// Two contracts with the same meaning have the same canonical form, so the
// pin does not depend on formatting.
func (m *Manifest) Canonical() string {
	parts := []string{
		"v=" + strconv.Itoa(m.Version),
		"candidate=" + m.Candidate,
		"baseline=" + m.Baseline,
		"metric=" + m.Metric,
		"judge=" + m.Judge,
		"selection=" + m.Selection,
		"heldout=" + m.Heldout,
		"candidates=" + strconv.Itoa(m.Candidates),
		"corr=" + formatFloat(m.Corr),
		"alpha=" + formatFloat(m.Alpha),
		"min_gain=" + formatFloat(m.MinGain),
		"power=" + formatFloat(m.Power),
		"locale=" + strings.Join(m.Locales, ";"),
	}
	if m.Note != "" {
		parts = append(parts, "note="+quoteIfNeeded(m.Note))
	}
	return strings.Join(parts, ", ")
}

// Hash returns the SHA-256 of the canonical form, hex encoded.
//
// This is the pin of the measurement contract.
func (m *Manifest) Hash() string {
	sum := sha256.Sum256([]byte(m.Canonical()))
	return hex.EncodeToString(sum[:])
}

// formatFloat renders a float without an exponent and without trailing zeros.
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}
	return s
}

// quoteIfNeeded quotes a value that contains characters the grammar reserves.
func quoteIfNeeded(v string) string {
	if v == "" {
		return `""`
	}
	if strings.ContainsAny(v, ",=#\"") || strings.TrimSpace(v) != v {
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(v) + `"`
	}
	return v
}
