---
id: AM-fuzz-finding-still-fails-build
type: automated-measure
title: Fuzz CI step must still fail on a real finding, not just on any non-zero exit
description: 'The fuzz step classifies its failures instead of trusting the exit code: it fails only when the output contains ''Failing input written to'' (the reproducer evidence a genuine finding always writes), tolerates ''context deadline exceeded'' with a ::notice:: (an expired -fuzztime budget on a slow runner, non-zero since Go 1.25), and still fails loudly on anything else (build errors, panics). This is what makes tolerating the timeout safe rather than a blanket ignore. The measure is the classification itself plus the recorded verification that a real crash exits 1 — tested with synthetic crash output, a build-error case, and a live always-failing target in a scratch module.'
kind: test
location: .github/workflows/ci.yml (fuzz job, run_fuzz helper); verified against a live always-failing fuzz target
status: active
---

## What this prevents

Two failure modes, and the second is the dangerous one:

1. **The flake** — a `-fuzztime` budget expiring fails the job on PRs that touch no fuzzed code, costing re-runs and training reviewers to dismiss red Fuzz results.
2. **The over-correction** — "just ignore fuzz failures" would silence genuine findings. A fuzzer that cannot fail the build is decoration.

The classification keeps both properties at once: timeouts are tolerated,
findings are not.

## The trap this measure encodes

The timeout output contains **both** `--- FAIL: <target>` and `context deadline
exceeded`. Matching a generic failure marker like `--- FAIL` would therefore
re-fail exactly the case being tolerated — a fix that looks correct and does
nothing. The crash check must be a **positive match on reproducer evidence**
(`Failing input written to`), never on a generic failure string.

A comment at the call site states this so a future editor doesn't "tidy" the
pattern and silently reintroduce the flake.

## Verification performed

| Input | Expected | Actual |
|---|---|---|
| Real captured CI timeout output | tolerate | exit 0, `::notice::` |
| Crash output with a reproducer path | fail | exit 1, `::error::` |
| **Live always-failing fuzz target** (scratch module) | fail | exit 1 |
| Build-error output | fail | exit 1, `::error::` unrecognised |
| All three production targets | pass | pass |

## Residual

This tolerates a *budget expiry*, which means a genuinely slow runner explores
less of the input space than intended — coverage degrades silently rather than
failing. That is the accepted trade: a fuzz run that explores less is still
strictly better than a gate everyone ignores. If exploration depth becomes a
concern, the lever is `-fuzztime` or a dedicated runner, not this
classification.
