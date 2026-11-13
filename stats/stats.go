// Package stats holds the small statistical primitives HELD needs.
//
// Everything here is deterministic and dependency-free. The normal quantile
// function is Acklam's rational approximation refined by one Halley step,
// accurate to about 1e-15 relative error. The expected maximum of N standard
// normals uses Blom's approximation of the expected maximum order statistic,
// which is within a few thousandths of the exact value for N >= 2.
//
// The package deliberately exposes no global state and no randomness. The
// synthetic benchmark driver keeps its own generator so that a published
// result can be reproduced byte for byte on any machine and any Go release.
package stats

import "math"

// NormInv returns the standard normal quantile function: the x such that
// P(Z <= x) = p, for 0 < p < 1.
//
// The function is defined by its own property, and it is computed that way
// rather than from a table of memorised coefficients. A bisection over the
// interval where the normal distribution has support converges to the double
// precision limit, and the accuracy of the result is limited only by the
// accuracy of NormCdf, which is the Go standard library's error function.
//
// The practical range is 1e-15 < p < 1-1e-15. Outside it the normal
// distribution underflows and the result saturates.
func NormInv(p float64) float64 {
	if p <= 0 || p >= 1 || math.IsNaN(p) {
		return math.NaN()
	}
	if p == 0.5 {
		return 0
	}
	// Solve in the lower half and mirror, so that the comparison stays in the
	// region where the complementary error function keeps its precision.
	sign := 1.0
	target := p
	if p > 0.5 {
		sign = -1
		target = 1 - p
	}
	lo, hi := -40.0, 0.0
	for i := 0; i < 200; i++ {
		mid := 0.5 * (lo + hi)
		if NormCdf(mid) < target {
			lo = mid
		} else {
			hi = mid
		}
		if hi-lo <= 1e-16 {
			break
		}
	}
	return sign * 0.5 * (lo + hi)
}

// NormCdf returns the standard normal cumulative distribution function.
//
// It uses the Chebyshev rational approximation for the complementary error
// function, whose fractional error stays below 1.2e-7 over the whole range.
func NormCdf(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}

// TwoSidedZ returns the two-sided critical value z_{1-alpha/2}.
func TwoSidedZ(alpha float64) float64 {
	return NormInv(1 - alpha/2)
}

// EffectiveCandidates converts the number of candidates produced in a round
// into the number that behave independently.
//
// Candidates in one round are usually correlated: they share a model family, a
// prompt scaffold and an evaluation set, so their errors move together. With a
// correlation rho the effective count is 1 + (N-1)*rho, which interpolates
// between a single candidate (rho = 1) and fully independent candidates
// (rho = 0).
func EffectiveCandidates(n int, rho float64) int {
	if n < 1 {
		return 1
	}
	if rho <= 0 {
		return n
	}
	if rho >= 1 {
		return 1
	}
	effective := 1 + float64(n-1)*rho
	// Round up so that the effective count never understates the selection.
	rounded := int(math.Ceil(effective))
	if rounded < 1 {
		rounded = 1
	}
	return rounded
}

// ExpectedMaxNormal returns the expected value of the maximum of n
// independent standard normal variables.
//
// This is the multiplier behind the winner's curse: when the best of n
// candidates is selected on a noisy measurement, its score sits above the
// truth by about this many standard errors. For n = 1 the answer is 0, for
// n = 2 it is 1/sqrt(pi) = 0.5642, and it grows like sqrt(2*ln(n)).
//
// Blom's approximation is used: the expected maximum order statistic of n
// normals is close to the normal quantile at (n - 0.375)/(n + 0.25).
func ExpectedMaxNormal(n int, rho float64) float64 {
	effective := EffectiveCandidates(n, rho)
	if effective <= 1 {
		return 0
	}
	q := (float64(effective) - 0.375) / (float64(effective) + 0.25)
	return NormInv(q)
}

// Mean returns the arithmetic mean of xs and whether xs was non-empty.
func Mean(xs []float64) (float64, bool) {
	if len(xs) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs)), true
}

// StdDev returns the sample standard deviation of xs (n-1 denominator) and
// whether there were at least two values.
func StdDev(xs []float64) (float64, bool) {
	if len(xs) < 2 {
		return 0, false
	}
	m, _ := Mean(xs)
	sum := 0.0
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(xs)-1)), true
}

// StdError returns the standard error of the mean of xs.
func StdError(xs []float64) (float64, bool) {
	if len(xs) < 2 {
		return 0, false
	}
	sd, _ := StdDev(xs)
	return sd / math.Sqrt(float64(len(xs))), true
}

// RequiredPairedN returns the number of paired held-out items needed to
// detect a true difference of delta with the given significance level and
// power, when the per-item standard deviation of the paired difference is
// sigma.
//
// It solves the standard two-sided normal approximation
//
//	n = ((z_{1-alpha/2} + z_{power}) * sigma / delta)^2
//
// The result is rounded up. The caller is responsible for choosing a
// realistic sigma; a binary success metric has a per-item standard deviation
// of at most 1/sqrt(2).
func RequiredPairedN(delta, alpha, power, sigma float64) (int, bool) {
	if delta <= 0 || sigma <= 0 || alpha <= 0 || alpha >= 1 || power <= 0 || power >= 1 {
		return 0, false
	}
	z := TwoSidedZ(alpha) + NormInv(power)
	n := math.Ceil((z * sigma / delta) * (z * sigma / delta))
	if n < 2 {
		n = 2
	}
	return int(n), true
}

// MaxPairedStdDev is the largest possible per-item standard deviation of a
// paired difference between two binary outcomes.
//
// The difference of two Bernoulli variables takes the values -1, 0 and 1, so
// its variance is at most 1/2 and its standard deviation at most 1/sqrt(2).
// Using this value gives a conservative sample size for binary metrics.
func MaxPairedStdDev() float64 {
	return 1 / math.Sqrt2
}
