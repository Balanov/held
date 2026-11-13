package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "held-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	name := "held"
	if runtime.GOOS == "windows" {
		name = "held.exe"
	}
	binary = filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// run executes the binary and returns its stdout, stderr and exit code.
func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

// demo writes the example contract and scores into a directory.
func demo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "measurement.held")
	scoresPath := filepath.Join(dir, "scores.jsonl")
	if err := os.WriteFile(contractPath, []byte(demoContract+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run(t, "demo", "--out", scoresPath); code != 0 {
		t.Fatalf("demo failed with code %d", code)
	}
	return contractPath, scoresPath
}

func TestVersion(t *testing.T) {
	out, _, code := run(t, "version")
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out, "held") {
		t.Errorf("output = %q", out)
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.held")
	if _, _, code := run(t, "init", "--out", path); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "v=1") {
		t.Errorf("written contract = %q", raw)
	}
}

func TestCheckAcceptsAndPins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.held")
	os.WriteFile(path, []byte(demoContract), 0o644)
	out, _, code := run(t, "check", path)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out, "pin ") {
		t.Errorf("output = %q", out)
	}
}

func TestCheckRejectsBadContract(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.held")
	os.WriteFile(path, []byte("v=1, candidate=c"), 0o644)
	_, errOut, code := run(t, "check", path)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid manifest") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestGateOnDemoData(t *testing.T) {
	contract, scores := demo(t)
	out, _, code := run(t, "gate", contract, "--data", scores)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for a conflicting claim", code)
	}
	if !strings.Contains(out, "CONFLICTING") {
		t.Errorf("output = %q", out)
	}
	if !strings.Contains(out, "CLAIMED GAIN") {
		t.Errorf("output = %q", out)
	}
}

func TestGateJSONOutput(t *testing.T) {
	contract, scores := demo(t)
	out, _, code := run(t, "gate", contract, "--data", scores, "--json")
	if code != 1 {
		t.Errorf("exit code = %d", code)
	}
	for _, key := range []string{`"verdict"`, `"pin"`, `"claimed_gain"`, `"heldout_gain"`} {
		if !strings.Contains(out, key) {
			t.Errorf("JSON output is missing %s: %s", key, out)
		}
	}
}

func TestGateRussianOutput(t *testing.T) {
	contract, scores := demo(t)
	out, _, _ := run(t, "gate", contract, "--data", scores, "--lang", "ru")
	if !strings.Contains(out, "ПРОТОКОЛ ИЗМЕРЕНИЯ") {
		t.Errorf("output = %q", out)
	}
	if !strings.Contains(out, "ПРОТИВОРЕЧИЕ") {
		t.Errorf("output = %q", out)
	}
}

func TestRenderShowsEveryLocale(t *testing.T) {
	contract, scores := demo(t)
	out, _, code := run(t, "render", contract, "--data", scores)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	for _, marker := range []string{"MEASUREMENT PROTOCOL", "PROTOCOLO DE MEDICIÓN", "测量协议", "ПРОТОКОЛ ИЗМЕРЕНИЯ"} {
		if !strings.Contains(out, marker) {
			t.Errorf("rendering is missing %q", marker)
		}
	}
}

func TestSealIsDeterministic(t *testing.T) {
	contract, scores := demo(t)
	first, _, code := run(t, "seal", contract, "--data", scores)
	if code != 1 {
		t.Errorf("exit code = %d", code)
	}
	second, _, _ := run(t, "seal", contract, "--data", scores)
	if first != second {
		t.Error("the receipt is not deterministic")
	}
	if !strings.Contains(first, "data_sha256") {
		t.Errorf("receipt = %q", first)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, _, code := run(t, "nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestGateWithoutData(t *testing.T) {
	contract, _ := demo(t)
	_, _, code := run(t, "gate", contract)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}
