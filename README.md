# HELD

> Measure on what you did not select.

HELD decides whether a claimed improvement in an AI system or a robot policy is
real. It measures the candidate against the baseline on data that was **never
used to pick the candidate**, corrects the selection-split gain for the
winner's curse, and tells you how much held-out evidence the claim would need
in order to be provable.

**One static binary. Zero dependencies. Offline. MIT.**

```
held gate measurement.held --data scores.jsonl
```

```
VERDICT                CONFLICTING
                       the selection split reports +0.1875, but the held-out
                       lower bound -0.0437 does not clear 0.0100

CLAIMED GAIN           +0.1875
EXPECTED SELECTION INFLATION 0.1783  (16 candidates, expected maximum 1.7688)
GAIN AFTER SELECTION CORRECTION +0.0092
HELD-OUT GAIN          +0.0000
HELD-OUT INTERVAL      [-0.0437, +0.0437]
HELD-OUT ITEMS NEEDED  2492
```

## Why

A self-improving loop proposes candidates, keeps the one that scores best on a
small evaluation set, and reports the score it got there. That number is
inflated, and the inflation grows with the number of candidates and shrinks
only slowly with the size of the evaluation set.

The published record on this is unambiguous:

| Source | Finding |
|---|---|
| [arXiv 2610.09239](https://arxiv.org/abs/2610.09239) | In a pre-registered study, the greedy selection-set score of self-improvement loops exceeded held-out accuracy by **13–20 points** with 16 selection items, and by 1–5 points with 256. Most proposals after the first were harmful. |
| [arXiv 2607.25152](https://arxiv.org/abs/2607.25152) | Across 54 cycles a frontier agent claimed improvement every time, yet **56 %** of the measured deltas were zero or negative. The self-verdict gate degenerated into accept-all and erased 19 % of the best deployed state. The strongest in-band judge accepted cycles of which **44 %** were real regressions. |
| [arXiv 2609.31760](https://arxiv.org/abs/2609.31760) | After 123 rounds of robot skill discovery, changes kept passing their own tests while the target task never succeeded: *the agent optimized exactly what the evaluator measured, including where it was wrong.* |
| [arXiv 2608.20290](https://arxiv.org/abs/2608.20290) | Seven measurement failures, each of which inverted a finding without a frozen control. |

HELD is the missing primitive: a frozen control, and a gate that reads it.

Nothing in the existing toolchain closes this. Eval harnesses report means and
confidence intervals but do not correct for the fact that the candidate was
*selected* on the very set they report. Experiment trackers record parameters
but do not decide whether the improvement is real. Guardrail libraries police
what a model may call, not whether its gains survive a fresh set.

## What it does

- **Gates a claim.** The verdict comes from the held-out interval alone. A
  candidate is accepted only when the lower end of that interval clears the
  declared minimum gain.
- **Corrects the winner's curse.** The gain the loop reports on the selection
  set is inflated by roughly one standard error times the expected maximum of
  the candidates that were produced. HELD reports the inflation and the gain
  that remains after removing it.
- **Names the conflict.** A candidate that looks better on the selection set
  but does not clear the held-out bar is reported as `conflicting`, not as
  `not_proven`, because that is the exact shape of a loop that has fooled
  itself.
- **Plans the evidence.** `held plan` computes how many held-out items a
  declared gain needs at a declared significance level and power. Proving a
  one point improvement on a binary metric needs about **39 000** paired items.
- **Pins the contract.** One comma-separated line declares the whole
  measurement protocol. Its SHA-256 is the pin, so a claim cannot be
  re-described after the fact without breaking the pin.
- **Speaks four languages.** The same contract renders into English, Russian,
  Spanish and Chinese from one source, so a number cannot drift between
  translations.

## Install

```sh
go install github.com/Balanov/held/cmd/held@latest
```

Or download a binary for your platform from
[the releases page](https://github.com/Balanov/held/releases/latest).

No runtime dependencies, no account, no telemetry, no network access.

## Use it

Write a contract. It is one line of clauses separated by commas:

```sh
held init --out measurement.held
```

```
v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate,
judge=state_check, selection=sel, heldout=hold, candidates=16,
alpha=0.05, min_gain=0.01, power=0.8, locale=en;ru;es;zh
```

Every clause is `key=value`. A value may be quoted, and a quoted value may
contain commas. A semicolon separates the elements of a list. A hash starts a
comment. Newlines are whitespace, so a long contract may be wrapped after any
comma and still parse to exactly the same contract.

Point it at your scores. One record is one outcome of one system on one item:

```json
{"split":"hold","system":"policy-v7","item":"h001","score":1}
```

```sh
held gate measurement.held --data scores.jsonl
```

Exit codes are `0` for a proven claim, `1` when the claim is not proven or
conflicts with the held-out measurement, and `2` for bad input. The gate is
therefore a CI step without a wrapper script:

```yaml
- name: Hold-out gate
  run: held gate measurement.held --data scores.jsonl
```

Try it without any data of your own:

```sh
held demo --out scores.jsonl
held gate measurement.held --data scores.jsonl
```

## Commands

| Command | What it does |
|---|---|
| `held init` | Write a starter measurement contract |
| `held check FILE` | Validate a contract and print its pin |
| `held plan FILE` | How much held-out evidence a declared gain needs |
| `held gate FILE --data FILE` | Decide whether the claim holds |
| `held render FILE` | Render the protocol in every language |
| `held seal FILE --data FILE` | Write a signed receipt |
| `held demo` | Write the example scores |

`held gate --json` prints a machine-readable receipt. `held seal` writes the
same numbers as a pinned receipt that includes the SHA-256 of the data file.

## The measurements

On a synthetic benchmark with a known truth, 2 000 rounds per configuration,
16 selection items and 16 candidates per round:

| Acceptance rule | False accept rate on a claim with no real improvement |
|---|---|
| Greedy: keep the candidate if it scored better | **89.9 %** |
| A significance test on the selection set | **23.9 %** |
| HELD: the held-out interval | **2.3 %** |

The nominal error rate is 5 %. Greedy acceptance is wrong about nine rounds in
ten. A plain significance test on the selection set is invalid after a
selection and is wrong about one round in four.

HELD pays for that honesty in power: with 1 000 held-out items it will refuse
to certify a five point gain, because a thousand items cannot prove five
points. `held plan` tells you what would.

Full numbers, the method and the raw JSON are in
[BENCHMARKS.md](BENCHMARKS.md) and in [`bench/results/`](bench/results/).

## Boundaries

- HELD does not run your model, your agent or your robot. It reads outcomes.
- It does not tell you *why* a candidate did not improve. It tells you that the
  improvement is not established.
- The selection-bias correction assumes the candidates share a selection set.
  When they also share a baseline, the true inflation is smaller than the
  predicted one, so the correction is conservative. The benchmark measures both.
- The sample-size plan assumes a normal approximation for a paired difference.
  It is a planning aid, not a substitute for a power analysis on your own data.
- The benchmark is synthetic. It reproduces the mechanism, not the published
  numbers of any particular paper.

## Cite it

```bibtex
@misc{balanov2026held,
  title  = {HELD: Held-out Measurement for Claimed AI and Robot Improvements},
  author = {Balanov, Anton},
  year   = {2026},
  url    = {https://balanov.pro/held/}
}
```

## License

MIT. Fork it, change it, ship it. See [LICENSE](LICENSE).

[Русский](README_RU.md) · [balanov.pro/held](https://balanov.pro/held/)
