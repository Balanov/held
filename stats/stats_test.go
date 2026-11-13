package stats

import (
	"math"
	"testing"
)

func TestNormInvKnownQuantiles(t *testing.T) {
	cases := []struct {
		p    float64
		want float64
		tol  float64
	}{
		{0.5, 0, 1e-12},
		{0.975, 1.959963984540054, 1e-9},
		{0.95, 1.6448536269514722, 1e-9},
		{0.8413447460685429, 1.0, 1e-9},
		{0.025, -1.959963984540054, 1e-9},
		{0.99, 2.3263478740408408, 1e-9},
	}
	for _, c := range cases {
		got := NormInv(c.p)
		if math.Abs(got-c.want) > c.tol {
			t.Errorf("NormInv(%v) = %.15f, want %.15f", c.p, got, c.want)
		}
	}
}

func TestNormInvRoundTrip(t *testing.T) {
	for _, p := range []float64{0.001, 0.01, 0.1, 0.3, 0.5, 0.7, 0.9, 0.99, 0.999} {
		x := NormInv(p)
		back := NormCdf(x)
		if math.Abs(back-p) > 1e-9 {
			t.Errorf("NormCdf(NormInv(%v)) = %.12f", p, back)
		}
	}
}

func TestNormInvRejectsOutOfRange(t *testing.T) {
	for _, p := range []float64{0, 1, -0.1, 1.1} {
		if !math.IsNaN(NormInv(p)) {
			t.Errorf("NormInv(%v) should be NaN", p)
		}
	}
}

func TestTwoSidedZ(t *testing.T) {
	if got := TwoSidedZ(0.05); math.Abs(got-1.959963984540054) > 1e-10 {
		t.Errorf("TwoSidedZ(0.05) = %.12f", got)
	}
}

func TestExpectedMaxNormal(t *testing.T) {
	if got := ExpectedMaxNormal(1, 0); got != 0 {
		t.Errorf("ExpectedMaxNormal(1) = %v, want 0", got)
	}
	// The exact expected maximum of two standard normals is 1/sqrt(pi).
	if got := ExpectedMaxNormal(2, 0); math.Abs(got-0.5641895835477563) > 0.03 {
		t.Errorf("ExpectedMaxNormal(2) = %.6f, want about 0.564190", got)
	}
	// For sixteen candidates the exact value is about 1.7655.
	if got := ExpectedMaxNormal(16, 0); math.Abs(got-1.7655) > 0.01 {
		t.Errorf("ExpectedMaxNormal(16) = %.6f, want about 1.7655", got)
	}
	// The maximum grows slowly, like the square root of twice the log.
	if got := ExpectedMaxNormal(10000, 0); math.Abs(got-3.858) > 0.05 {
		t.Errorf("ExpectedMaxNormal(10000) = %.6f, want about 3.858", got)
	}
}

func TestExpectedMaxNormalCorrelation(t *testing.T) {
	// Fully correlated candidates behave like a single candidate.
	if got := ExpectedMaxNormal(16, 1); got != 0 {
		t.Errorf("ExpectedMaxNormal(16, rho=1) = %v, want 0", got)
	}
	// Half correlation sits between one candidate and sixteen independent ones.
	mid := ExpectedMaxNormal(16, 0.5)
	one := ExpectedMaxNormal(16, 1)
	full := ExpectedMaxNormal(16, 0)
	if !(one <= mid && mid <= full) {
		t.Errorf("correlation ordering broken: %v, %v, %v", one, mid, full)
	}
}

func TestEffectiveCandidates(t *testing.T) {
	if got := EffectiveCandidates(16, 0); got != 16 {
		t.Errorf("EffectiveCandidates(16, 0) = %d", got)
	}
	if got := EffectiveCandidates(16, 1); got != 1 {
		t.Errorf("EffectiveCandidates(16, 1) = %d", got)
	}
	if got := EffectiveCandidates(16, 0.5); got != 9 {
		t.Errorf("EffectiveCandidates(16, 0.5) = %d, want 9", got)
	}
}

func TestMeanStdDevStdError(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	m, ok := Mean(xs)
	if !ok || m != 3 {
		t.Errorf("Mean = %v, %v", m, ok)
	}
	sd, ok := StdDev(xs)
	if !ok || math.Abs(sd-1.5811388300841898) > 1e-12 {
		t.Errorf("StdDev = %v", sd)
	}
	se, ok := StdError(xs)
	if !ok || math.Abs(se-sd/math.Sqrt(5)) > 1e-12 {
		t.Errorf("StdError = %v", se)
	}
	if _, ok := Mean(nil); ok {
		t.Error("Mean of nothing should not report ok")
	}
	if _, ok := StdDev([]float64{1}); ok {
		t.Error("StdDev of one value should not report ok")
	}
}

func TestRequiredPairedN(t *testing.T) {
	// A one point improvement on a binary metric, at 95 percent confidence and
	// 80 percent power, needs tens of thousands of paired items.
	n, ok := RequiredPairedN(0.01, 0.05, 0.8, MaxPairedStdDev())
	if !ok {
		t.Fatal("RequiredPairedN failed")
	}
	if n < 38000 || n > 40000 {
		t.Errorf("RequiredPairedN = %d, want about 39241", n)
	}
	// A ten point improvement needs about a hundred times fewer items.
	n2, ok := RequiredPairedN(0.10, 0.05, 0.8, MaxPairedStdDev())
	if !ok {
		t.Fatal("RequiredPairedN failed")
	}
	if n2 > n/50 {
		t.Errorf("RequiredPairedN for a ten point gain = %d, want far below %d", n2, n)
	}
	if _, ok := RequiredPairedN(0, 0.05, 0.8, 1); ok {
		t.Error("a zero delta should not be accepted")
	}
	if _, ok := RequiredPairedN(0.01, 1.5, 0.8, 1); ok {
		t.Error("an alpha above one should not be accepted")
	}
}

func TestMaxPairedStdDev(t *testing.T) {
	if math.Abs(MaxPairedStdDev()-0.7071067811865476) > 1e-12 {
		t.Errorf("MaxPairedStdDev = %v", MaxPairedStdDev())
	}
}
