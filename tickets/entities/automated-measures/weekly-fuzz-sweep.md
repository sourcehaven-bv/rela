---
id: weekly-fuzz-sweep
type: automated-measure
title: Weekly fuzz sweep over all fuzz targets
description: Scheduled weekly CI workflow that discovers and fuzzes every Fuzz* target for 25s each; failures upload crashing inputs as artifacts and auto-file a deduped GitHub issue (label fuzz-failure). Guards against fuzz-reachable bugs that the per-PR 3-target smoke job never exercises.
kind: ci
location: .github/workflows/fuzz-sweep.yml
status: active
---

Scheduled CI workflow (`.github/workflows/fuzz-sweep.yml`, Mondays 06:00 UTC)
that discovers and runs every `Fuzz*` target in the repo for 25s each via
`scripts/fuzz-all.sh`. Failures upload the crashing inputs as artifacts and
auto-file a deduped GitHub issue (label `fuzz-failure`).

Introduced by TKT-PCLGGL. Two seconds of its first local run found five failing
targets: four stale fuzz-harness oracles (fixed in the same PR — the storetest
collision harnesses now delegate ID validity to `storeutil.ValidateID`
directionally) and one real production bug (BUG-RHFHTH, GenerateShortID emitting
validator-rejected IDs).

Local equivalent: `just fuzz-all` (or `just fuzz-all fuzztime=5s`).
