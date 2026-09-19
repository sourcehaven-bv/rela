---
id: RR-SEIKCR
type: review-response
title: migrate data guessed the stored shape on the un-baselined path, replanning the entire chain
finding: 'MigrateDataCmd.Run set `stored = files[0].FromProjection` when nothing was recorded, so Resolve had something to compare against. In the un-baselined case the store''s real shape is UNKNOWN - that is exactly why StatusUnbaselined exists and why the gate refuses to guess. A store already at the chain''s end (record lost, or gitignored by accident) has every migration replanned and re-applied to migrated content. The residual compatibility check cannot catch it: with every file planned, the walk ends at the last file''s to-shape, which equals live by construction, so the one diagnostic that would notice is structurally blind on exactly this path. Making the dangerous path the DEFAULT behaviour of migrate data inverted the un-baselined refusal the ticket introduced.'
severity: critical
resolution: 'Fixed: migrate data now refuses when migrations exist and nothing is recorded, printing what it found and pointing at `rela migrate baseline --apply`, exiting non-zero. Pinned by TestMigrateData_RefusesToGuessWhenUnbaselined. Verified end to end against a real project: losing applied.json now refuses instead of silently replaying. Docs updated - every command refuses, not just status.'
status: addressed
---

## Finding

`MigrateDataCmd.Run` set `stored = files[0].FromProjection` when nothing was
recorded, so `Resolve` had a baseline to compare against.

In the un-baselined case the store's real shape is *unknown* — that is precisely
why `StatusUnbaselined` exists and why the gate refuses to guess. This line
guessed anyway.

## Failure scenario

A store whose content is already at the chain's end: someone ran the migrations
before `applied.json` existed, or it was gitignored by accident.

1. `files[0].FromProjection` is the *first* migration's from-shape.
2. `Resolve` plans every file.
3. The residual check passes — with every file planned, the walk ends at the
last file's to-shape, which equals live **by construction**.
4. The whole chain re-runs against already-migrated content.

The residual check is supposed to be the diagnostic backstop, and it is
structurally blind on exactly the path that most needs one. A `map_values` or
`set_default` over already-correct data is not necessarily a no-op, and
`--apply` is one flag away from the dry-run the operator just read.

Making the dangerous path the **default** behaviour of `migrate data` inverted
the un-baselined refusal this ticket introduced.

## Severity

Critical. Same class as RR-QQLKDE and reachable without any upgrade — just a
lost or gitignored record.

## Resolution

`migrate data` now refuses when migrations exist and nothing is recorded. It
prints what it found, explains why it will not guess, points at `rela migrate
baseline --apply`, and exits non-zero.

Pinned by `TestMigrateData_RefusesToGuessWhenUnbaselined`. Several existing
tests had to start from a baselined store rather than the un-baselined one,
which is the realistic workflow anyway — a `baselineEmpty` helper makes that
explicit.

Verified end to end against a real project: deleting `applied.json` with a
migration present now refuses with actionable guidance, and `rela migrate
baseline --apply` resolves it.

The data-migration guide was updated: **every** command refuses, not just
`status`, and the upgrade-from-legacy path lands here too.
