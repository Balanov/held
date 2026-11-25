## [0.1.2] — 2026-11-25

### Fixed

- A synthetic worked example that produces the CONFLICTING verdict is in `examples/conflicting`, with its contract, its scores and the expected exit status. A test reads the checked-in files back and asserts the verdict.


## 0.1.1 - 2026-11-15

### Fixed

- A score record without a score, with a null score, or with an unknown key is refused, and the error names the line or the record index. An explicit zero is still a measured zero.
# Changelog

## 0.1.0 — 14 November 2026

First release.

- One-line comma-separated measurement contract, with a canonical form and a
  SHA-256 pin.
- Held-out gate: the verdict comes from the interval on data that was never
  used for selection.
- Selection-bias correction: the expected inflation of the selection-split gain
  and the gain that remains after removing it.
- Verdicts `proven`, `not_proven` and `conflicting`.
- `held plan`: the number of held-out items a declared gain needs.
- `held seal`: a pinned receipt with the SHA-256 of the contract and of the
  data file.
- Protocol rendering in English, Russian, Spanish and Chinese from one source.
- A synthetic benchmark with a known truth, a cross-check against the shipped
  gate and published raw results.
- One static binary, no runtime dependencies, offline, MIT.
