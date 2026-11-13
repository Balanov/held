// Command held measures claimed AI and robot improvements on data that was
// never used for selection.
//
// Usage:
//
//	held init [--out FILE]
//	held check FILE
//	held plan FILE [--data FILE]
//	held gate FILE --data FILE [--lang LOCALE] [--json]
//	held render FILE [--data FILE] [--lang LOCALE,LOCALE]
//	held seal FILE --data FILE [--out FILE]
//	held version
//
// Exit codes are 0 for a proven claim, 1 when the claim is not proven or
// conflicts with the held-out measurement, and 2 for bad input. The gate is
// therefore usable as a CI step without a wrapper script.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Balanov/held/gate"
	"github.com/Balanov/held/i18n"
	"github.com/Balanov/held/manifest"
	"github.com/Balanov/held/report"
	"github.com/Balanov/held/scores"
)

// Version is the release version of the tool.
const Version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
	case "plan":
		err = cmdPlan(os.Args[2:])
	case "gate":
		err = cmdGate(os.Args[2:])
	case "render":
		err = cmdRender(os.Args[2:])
	case "seal":
		err = cmdSeal(os.Args[2:])
	case "demo":
		err = cmdDemo(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("held " + Version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "held: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		if code, ok := err.(exitError); ok {
			os.Exit(code.code)
		}
		fmt.Fprintln(os.Stderr, "held: "+err.Error())
		os.Exit(2)
	}
}

// exitError carries a specific process exit code.
type exitError struct {
	code int
}

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func usage() {
	fmt.Fprint(os.Stderr, `held - measure claimed improvements on data never used for selection

  held init   [--out FILE]            write a starter measurement contract
  held check  FILE                     validate a contract and print its pin
  held plan   FILE [--data FILE]       how much held-out evidence a gain needs
  held gate   FILE --data FILE         decide whether the claim holds
  held render FILE [--data FILE]       render the protocol in every language
  held seal   FILE --data FILE         write a signed receipt
  held version

Exit codes: 0 proven, 1 not proven or conflicting, 2 bad input.
`)
}

// readManifest loads a contract from a path or from standard input.
func readManifest(path string) (*manifest.Manifest, error) {
	var raw []byte
	var err error
	if path == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return manifest.Parse(string(raw))
}

// demoContract is the example measurement contract shipped with the tool.
const demoContract = "v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate, " +
	"judge=state_check, selection=sel, heldout=hold, candidates=16, alpha=0.05, " +
	"min_gain=0.01, locale=en;ru;es;zh"

// demoBaseline and demoCandidate are the system names used by the example
// scores, so that the starter contract and the example data fit together.
const (
	demoBaseline  = "policy-v3"
	demoCandidate = "policy-v7"
)

// cmdDemo writes the example scores file.
//
// The example is built so that the story is visible in the numbers: the
// candidate wins three of sixteen selection items and reports a gain of nearly
// nineteen per cent, while on the held-out items it wins one and loses one,
// so the real gain is zero. HELD calls that a conflict, and greedy acceptance
// would have shipped it.
// boolFlag is implemented by flag values that do not consume the next
// argument.
type boolFlag interface {
	IsBoolFlag() bool
}

// parseFlags parses flags that may appear before, between or after the
// positional arguments.
//
// The standard flag package stops at the first positional argument, which
// makes "held gate FILE --data FILE" fail. This pass separates the two kinds
// of argument so that either order works.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			flagArgs = append(flagArgs, a)
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") {
				continue
			}
			f := fs.Lookup(name)
			if f == nil {
				continue
			}
			if b, ok := f.Value.(boolFlag); ok && b.IsBoolFlag() {
				continue
			}
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}
	return positional, nil
}

// writeFile writes data to path, creating the parent directory if needed.
func writeFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func cmdDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	out := fs.String("out", "scores.jsonl", "where to write the example scores")
	fs.Parse(args)

	baseSel := map[int]bool{0: true, 1: true, 2: true, 3: true, 8: true, 9: true, 12: true}
	candSel := map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true, 5: true,
		8: true, 9: true, 10: true, 12: true}
	baseHold := map[int]bool{}
	for i := 0; i < 32; i++ {
		baseHold[i] = true
	}
	candHold := map[int]bool{}
	for i := 0; i < 31; i++ {
		candHold[i] = true
	}
	candHold[32] = true

	var b strings.Builder
	emit := func(split, system, item string, v bool) {
		score := 0.0
		if v {
			score = 1
		}
		fmt.Fprintf(&b, "{\"split\":%q,\"system\":%q,\"item\":%q,\"score\":%v}\n", split, system, item, score)
	}
	for i := 0; i < 16; i++ {
		item := fmt.Sprintf("s%03d", i)
		emit("sel", demoBaseline, item, baseSel[i])
		emit("sel", demoCandidate, item, candSel[i])
	}
	for i := 0; i < 64; i++ {
		item := fmt.Sprintf("h%03d", i)
		emit("hold", demoBaseline, item, baseHold[i])
		emit("hold", demoCandidate, item, candHold[i])
	}
	if err := writeFile(*out, []byte(b.String())); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", *out)
	fmt.Println("  run: held gate measurement.held --data " + *out)
	return nil
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	out := fs.String("out", "measurement.held", "where to write the contract")
	fs.Parse(args)

	text := "v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate, " +
		"judge=state_check, selection=sel, heldout=hold, candidates=16, " +
		"alpha=0.05, min_gain=0.01, power=0.8, locale=en;ru;es;zh\n"
	if err := writeFile(*out, []byte(text)); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", *out)
	fmt.Println("  edit it, then run: held gate " + *out + " --data scores.jsonl")
	return nil
}

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("check needs exactly one contract file")
	}
	m, err := readManifest(pos[0])
	if err != nil {
		return err
	}
	fmt.Println(m.Canonical())
	fmt.Println("pin " + m.Hash())
	return nil
}

func cmdPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	data := fs.String("data", "", "scores file used to estimate the per-item spread")
	lang := fs.String("lang", i18n.English, "output language")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("plan needs exactly one contract file")
	}
	m, err := readManifest(pos[0])
	if err != nil {
		return err
	}

	var heldDiffs []float64
	selectionItems := 0
	if *data != "" {
		d, err := scores.Load(*data)
		if err != nil {
			return err
		}
		diffs, _, err := d.Pair(m.Heldout, m.Baseline, m.Candidate)
		if err == nil {
			heldDiffs = diffs
		}
		if sel, _, err := d.Pair(m.Selection, m.Baseline, m.Candidate); err == nil {
			selectionItems = len(sel)
		}
	}

	p := gate.MakePlan(m, selectionItems, heldDiffs)
	t := func(key string) string { return i18n.T(*lang, key) }
	fmt.Printf("%s %d\n", t("required_heldout"), p.RequiredHeldout)
	fmt.Printf("%s %.4f  (%s)\n", t("sigma"), p.SigmaItem, t("sigma_source"))
	if selectionItems > 0 {
		fmt.Printf("%s %.4f  (%d %s, %s %.4f)\n", t("selection_inflation"), p.ExpectedInflation,
			p.Candidates, t("candidates"), t("expected_max"), p.ExpectedMax)
	}
	return nil
}

func cmdGate(args []string) error {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	data := fs.String("data", "", "scores file")
	lang := fs.String("lang", i18n.English, "output language")
	asJSON := fs.Bool("json", false, "print a machine-readable receipt")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("gate needs exactly one contract file")
	}
	if *data == "" {
		return fmt.Errorf("gate needs --data")
	}
	m, err := readManifest(pos[0])
	if err != nil {
		return err
	}
	d, err := scores.Load(*data)
	if err != nil {
		return err
	}
	res, err := gate.Evaluate(m, d)
	if err != nil {
		return err
	}

	if *asJSON {
		out := map[string]any{
			"contract":          m.Canonical(),
			"pin":               m.Hash(),
			"verdict":           res.Verdict,
			"claimed_gain":      res.ClaimedGain,
			"selection_inflation": res.Inflation,
			"corrected_claim":   res.CorrectedClaim,
			"heldout_gain":      res.HeldoutGain,
			"lower_bound":       res.LowerBound,
			"upper_bound":       res.UpperBound,
			"selection_items":   res.Selection.N,
			"heldout_items":     res.Heldout.N,
			"required_heldout":  res.RequiredHeldout,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
	} else {
		fmt.Print(report.Protocol(m, res, *lang))
	}

	if res.Verdict != gate.VerdictProven {
		return exitError{code: 1}
	}
	return nil
}

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	data := fs.String("data", "", "scores file, optional")
	langs := fs.String("lang", "", "comma separated locales, default: the contract's own")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("render needs exactly one contract file")
	}
	m, err := readManifest(pos[0])
	if err != nil {
		return err
	}

	var res *gate.Result
	if *data != "" {
		d, err := scores.Load(*data)
		if err != nil {
			return err
		}
		res, err = gate.Evaluate(m, d)
		if err != nil {
			return err
		}
	}

	targets := m.Locales
	if *langs != "" {
		targets = nil
		for _, l := range strings.Split(*langs, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				targets = append(targets, l)
			}
		}
	}
	for _, l := range targets {
		if !i18n.Has(l) {
			return fmt.Errorf("unknown locale %q (known: %s)", l, strings.Join(i18n.Locales(), ", "))
		}
		fmt.Printf("──── %s ────\n", i18n.Describe(l))
		fmt.Println(report.Protocol(m, res, l))
	}
	return nil
}

// receipt is the machine-readable seal of one measurement.
type receipt struct {
	Contract        string  `json:"contract"`
	Pin             string  `json:"pin"`
	DataSHA256      string  `json:"data_sha256"`
	Verdict         string  `json:"verdict"`
	ClaimedGain     float64 `json:"claimed_gain"`
	SelectionInflation float64 `json:"selection_inflation"`
	CorrectedClaim  float64 `json:"corrected_claim"`
	HeldoutGain     float64 `json:"heldout_gain"`
	LowerBound      float64 `json:"lower_bound"`
	UpperBound      float64 `json:"upper_bound"`
	SelectionItems  int     `json:"selection_items"`
	HeldoutItems    int     `json:"heldout_items"`
	RequiredHeldout int     `json:"required_heldout"`
}

func cmdSeal(args []string) error {
	fs := flag.NewFlagSet("seal", flag.ContinueOnError)
	data := fs.String("data", "", "scores file")
	out := fs.String("out", "", "where to write the receipt, default: standard output")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("seal needs exactly one contract file")
	}
	if *data == "" {
		return fmt.Errorf("seal needs --data")
	}
	m, err := readManifest(pos[0])
	if err != nil {
		return err
	}
	d, err := scores.Load(*data)
	if err != nil {
		return err
	}
	res, err := gate.Evaluate(m, d)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(*data)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)

	r := receipt{
		Contract:           m.Canonical(),
		Pin:                m.Hash(),
		DataSHA256:         hex.EncodeToString(sum[:]),
		Verdict:            res.Verdict,
		ClaimedGain:        res.ClaimedGain,
		SelectionInflation: res.Inflation,
		CorrectedClaim:     res.CorrectedClaim,
		HeldoutGain:        res.HeldoutGain,
		LowerBound:         res.LowerBound,
		UpperBound:         res.UpperBound,
		SelectionItems:     res.Selection.N,
		HeldoutItems:       res.Heldout.N,
		RequiredHeldout:    res.RequiredHeldout,
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if *out == "" {
		os.Stdout.Write(encoded)
	} else {
		if err := writeFile(*out, encoded); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", *out)
	}
	if res.Verdict != gate.VerdictProven {
		return exitError{code: 1}
	}
	return nil
}
