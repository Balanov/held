# A conflicting claim

The synthetic scores claim a gain on the selection split but do not support it on the held-out split. The measurement names the same baseline, candidate and splits as the scores. No network or service is needed.

From the repository root:

```sh
go run ./cmd/held gate examples/conflicting/measurement.held --data examples/conflicting/scores.jsonl
```

Expected output:

```text
HELD · MEASUREMENT PROTOCOL
────────────────────────────────────────────────────────────────
CONTRACT                       v=1, candidate=policy-v7, baseline=policy-v3, metric=success_rate, judge=state_check, selection=sel, heldout=hold, candidates=16, corr=0, alpha=0.05, min_gain=0.01, power=0.8, locale=en;ru;es;zh
CONTRACT PIN                   66e2cdb8b8f79a76421ba1e05595a28226a4a5b218e67993d49ec5dc3c8c6838

CANDIDATE                      policy-v7
BASELINE                       policy-v3
METRIC                         success_rate
JUDGE                          state_check
SELECTION SPLIT                sel  16 items  used to pick the candidate
HELD-OUT SPLIT                 hold  64 items  never used to pick the candidate
CANDIDATES IN ROUND            16
SIGNIFICANCE                   0.0500
MINIMUM GAIN                   0.0100
TARGET POWER                   0.8000

VERDICT                        CONFLICTING
                       the selection split reports +0.1875, but the held-out lower bound -0.0437 does not clear 0.0100

CLAIMED GAIN                   +0.1875
EXPECTED SELECTION INFLATION   0.1783  (16 CANDIDATES IN ROUND, expected maximum 1.7688)
GAIN AFTER SELECTION CORRECTION +0.0092
HELD-OUT GAIN                  +0.0000
HELD-OUT INTERVAL              [-0.0437, +0.0437]
LOWER BOUND                    -0.0437
HELD-OUT ITEMS NEEDED          2492
PER-ITEM SPREAD                0.1782  (estimated from the held-out split)
```

The CLI exits 1 because the claim conflicts with held-out evidence. `go run` also prints `exit status 1`. To inspect the verdict field, add `--json`. This demonstrates a rejected claim; it is not a measured result from a real system.
