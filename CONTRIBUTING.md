# Contributing

HELD is small on purpose: one binary, no dependencies, no services. Contributions
that keep it that way are the most valuable ones.

## Development

```sh
go test ./...             # the whole suite
go vet ./...              # static checks
go run ./bench --rounds 2000 --out bench/results
```

The benchmark is the measurement. If you change the gate, the statistics or the
format, re-run it and update `BENCHMARKS.md` with the new numbers. A change
that makes the numbers worse is still a change worth making, but it has to be
visible in the table.

## Rules of the road

- **Zero runtime dependencies.** The standard library is enough.
- **Offline by default.** No network calls, no telemetry, no accounts.
- **Deterministic.** The same inputs must give the same outputs, byte for byte,
  on any machine and any Go release. That is why the benchmark uses its own
  SplitMix64 generator instead of `math/rand`.
- **Fail closed.** A missing split, a malformed contract or an unreachable input
  is an error, never a silent default and never a pass.
- **Honest boundaries.** A measurement that does not generalise says so in the
  README, in the benchmark notes and in the tool output.

## Reporting a measurement problem

If HELD reaches the wrong verdict on your data, open an issue with the contract,
the scores and the verdict you expected. A reduced case is worth more than a
description.

## Adding a language

The translation table lives in `i18n/i18n.go`. A locale is complete when every
key of the English table has a translation; the package test fails otherwise.
Add the locale to `Locales()`, to the `manifest` package's known list, and to
the site.

## Adding a command

Commands live in `cmd/held`. Flags may appear before or after the positional
argument; `parseFlags` handles that. Exit codes are part of the interface: `0`
proven, `1` not proven or conflicting, `2` bad input.
