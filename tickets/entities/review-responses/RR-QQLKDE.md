---
id: RR-QQLKDE
type: review-response
title: LegacyBridge silently drops unconvertible names, producing an empty applied list that replays the whole chain
finding: 'loadLegacy dropped any applied entry whose name failed ParseMigrationName, warned, and returned a *State anyway. Every legacy name is %04d-shaped, so under the old scheme EVERY entry fails and is dropped - the returned state carries a valid projection beside an EMPTY applied list. Resolve reads that as ''nothing has run'' and plans every migration, and the residual shape check passes because the projection already matches. Both migrations re-run against already-migrated data. The doc comment claimed the operator is told to baseline instead, but nothing enforced it: the code dropped and proceeded, so the warning was advisory and the replay automatic. Reproduced with a test before fixing.'
severity: critical
resolution: 'Fixed: an unconvertible name now refuses the WHOLE marker (returns nil), which the caller treats as absent - and with migrations present that is StatusUnbaselined, so the gate refuses to guess and the operator resolves it explicitly. Partial adoption is never safe here. Pinned by TestLegacyBridge_RefusesUnconvertibleLegacyNames with both an all-legacy and a mixed-names subtest, the mixed case being the dangerous one because a partial adoption looks plausible.'
status: addressed
---

## Finding

`LegacyBridge.loadLegacy` (`legacy.go:118-133`) dropped any applied entry whose
name failed `ParseMigrationName`, logged a warning, and returned a `*State`
anyway.

Every legacy name is `%04d-name.yaml`, so under the old scheme **every** entry
fails and is dropped. The resulting state has a valid projection and an **empty
applied list**.

## Failure scenario

1. An operator on the old build has `0001-rename.yaml` and `0002-backfill.yaml`
applied.
2. They upgrade and rename the files to the timestamp scheme — which the warning
tells them to do.
3. On the next `rela migrate data`, `LegacyBridge.Load` returns a state with
`Applied: []` and the current projection.
4. `Resolve` sees both files absent from the applied list and plans **both**.
The residual shape check passes, because the projection already matches.
5. Both migrations re-run against already-migrated data.

The doc comment claimed "Dropping it would replay the migration; the operator is
told to baseline instead" — but nothing *enforced* the baseline. The code
dropped the entry and proceeded, so the warning was advisory and the replay
automatic.

I reproduced this with a throwaway test before fixing: it returned a usable
state with `applied=[]`, confirming the reviewer's reading exactly.

## Severity

Critical. Silent data corruption on the upgrade path — the one path every
existing deployment takes — in a subsystem that transforms user content.

## Resolution

An unconvertible name now refuses the **whole** marker (`return nil, nil`). The
caller treats that as absent, and with migrations present that is
`StatusUnbaselined`: the gate refuses to guess and the operator resolves it
explicitly by renaming the files and running `rela migrate baseline --apply`.

Partial adoption is never safe here, because a partially-converted record is
indistinguishable from a genuinely-empty one at the point `Resolve` reads it.

Pinned by `TestLegacyBridge_RefusesUnconvertibleLegacyNames`, with both an
all-legacy and a mixed-names subtest — the mixed case being the more dangerous
one, because a partial adoption looks plausible.
