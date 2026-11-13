# Measurements

Every number on this page comes from the benchmark in [`bench/`](bench/), which
drives the shipped gate package rather than a reimplementation of it. The raw
results are in [`bench/results/benchmark.json`](bench/results/benchmark.json)
and the tables below are generated from them.

Reproduce with:

```sh
go run ./bench --rounds 2000 --out bench/results
```

**Environment.** Windows 11 / AMD64, Go 1.27.4, CPU only, no network. The
generator is a SplitMix64 stream with an explicit seed, so the numbers are
reproducible byte for byte on any machine. The standard library random
generator is deliberately not used, because its algorithm is not contractually
stable across Go releases.

**Design.** A round produces N candidates. The loop measures all of them on the
selection split and keeps the best. The winner is then measured on the
held-out split, which took no part in the selection. The truth is known by
construction, so the false-accept rate of every acceptance rule is measured
rather than argued about.

**Cross-check.** The harness and the shipped `gate` package are driven over
the same rounds. In the published run they disagreed on **0 of 201 rounds**.

---

## The headline

Null scenario: no candidate is actually better than the baseline, so every
acceptance is a false accept. 2 000 rounds per configuration, 1 000 held-out
items, significance 0.05.

| Selection items | Candidates | Greedy | Significance test on the selection set | HELD |
|---:|---:|---:|---:|---:|
| 16 | 16 | **89.90 %** | 23.90 % | **2.30 %** |
| 16 | 64 | **96.45 %** | 40.25 % | **2.20 %** |
| 256 | 16 | **93.05 %** | 18.90 % | **2.80 %** |

The nominal error rate is 5 %. Greedy acceptance is wrong about nine rounds in
ten. A plain significance test on the selection set is invalid after a
selection, because the selected score is not a fresh draw, and it is wrong
about one round in four. The held-out rule stays inside the nominal rate.

## What the loop reports versus what is true

Alternative scenario: every candidate is genuinely better by 5 points.
16 selection items, 16 candidates, minimum gain 0.

| Rule | Accept rate | False reject rate | Gain the loop reports | Real gain | Overstatement |
|---|---:|---:|---:|---:|---:|
| Greedy | 94.85 % | 5.15 % | **+24.87 points** | +5.00 points | **+19.87 points** |
| Significance test on the selection set | 34.90 % | 65.10 % | +24.87 points | +5.00 points | +19.87 points |
| HELD | 67.40 % | 32.60 % | +24.87 points | +5.00 points | +19.87 points |

Every rule sees the same rounds and the same reported number. The difference
is in what they conclude about it. Greedy accepts a candidate whose real
advantage is 5 points while telling the world it is 24.87.

## The selection correction

The correction predicts the inflation of the selection-split gain as the
standard error of the paired difference times the expected maximum of the
candidates that were produced.

| Selection items | Candidates | Predicted inflation | Observed overstatement |
|---:|---:|---:|---:|
| 16 | 16 | +28.10 points | +20.57 points |
| 16 | 64 | +36.09 points | +26.68 points |
| 256 | 16 | +7.32 points | +5.19 points |

The prediction is an upper bound, and the benchmark shows why: the candidates
in a round share the same baseline, which makes their errors move together and
reduces the spread of the maximum. The `corr` clause of the contract exists to
model that correlation explicitly. **The correction is conservative: it never
claims less inflation than was actually observed.**

## The price of honesty

Alternative scenario, minimum gain 5 points, 1 000 held-out items.

| Rule | Accept rate | False reject rate |
|---|---:|---:|
| Greedy | 95.45 % | 4.55 % |
| Significance test on the selection set | 22.70 % | 77.30 % |
| HELD | **2.40 %** | **97.60 %** |

HELD refuses. A thousand held-out items cannot prove a five point gain on a
noisy binary metric, and HELD says so instead of shipping the candidate. The
way out is not a cleverer rule, it is more evidence, and `held plan` says how
much:

```sh
$ held plan measurement.held
HELD-OUT ITEMS NEEDED  39241
```

That figure is the conservative one: it assumes the worst case for a paired
difference between two binary outcomes, a per-item standard deviation of
1/√2. A one point gain on a binary metric needs about **39 000** paired
held-out items at 95 % confidence and 80 % power. A ten point gain needs about
four hundred.

This is the trade HELD makes, and it is stated rather than hidden: **HELD will
not certify an improvement it cannot prove, and it will tell you exactly how
much data would prove it.**

## Method notes and limits

- The metric is binary with item difficulty drawn uniformly, so the per-item
  spread of the paired difference is close to the worst case.
- The held-out split is drawn fresh in each round. The reported rate is
  therefore the rate of a rule applied once to one round. A held-out set that
  is reused across rounds accumulates its own error, which is a reason to pin
  it, not to relax the gate.
- The benchmark is synthetic. It reproduces the mechanism of the selection
  bias, not the published numbers of any particular paper, and it says nothing
  about how a specific lab's evaluation set behaves.
- The selection-bias correction assumes candidates are independent unless the
  `corr` clause says otherwise. Independence is an upper bound on the
  inflation.
- The sample-size plan uses a normal approximation for a paired difference. It
  is a planning aid, not a substitute for a power analysis on real data.

## Tool measurements

| Measurement | Value |
|---|---|
| Runtime dependencies | 0 bytes |
| Binary size, Windows amd64, stripped | see [the release page](https://github.com/Balanov/held/releases/latest) |
| Contract parse and pin, 14 clauses | microseconds |
| Gate evaluation, 16 + 1 000 paired items | under a millisecond |
| Test suite | `go test ./...`, all packages pass |
| Cross-check between harness and shipped gate | 0 mismatches in 201 rounds |
