---
id: RR-I2FPYC
type: review-response
title: LegacyBridge returns adopted state without running ValidateState
finding: '[security] ValidateState is documented as THE on-the-way-in-from-storage check, and all three durable backends call it in Load. LegacyBridge.loadLegacy constructs a State by hand and returns it without that call, so it is a StateStore whose Load path skips validation. No exploit was constructible: names are parsed in the loop above, FormatVersion is hardcoded to the current constant, and a malformed projection is caught downstream by classifyAgainst (which correctly errors rather than re-baselining). This is the defect ValidateState''s own comment warns about - validating at the point of use leaves each caller to remember - one refactor away from mattering.'
severity: minor
resolution: 'Fixed: loadLegacy now calls ValidateState on the constructed state before returning it. A failure warns and returns (nil, nil), joining the single deliberate legacy fail-open rather than creating a second one - bootstrap is where an unreadable legacy marker already lands.'
status: addressed
---

## Finding

`ValidateState` is documented as the on-the-way-in-from-storage gate:

> Name validation happens HERE, on the way in from storage, because the
> applied-list is compared against directory entries and its names can reach a
> path join. Validating at the point of use instead would leave each caller to
> remember. (`state.go:158-164`)

All three durable backends honour that — `filemigstate`, `pgmigstate` and
`sqlitemigstate` each call it in `Load`. `LegacyBridge.loadLegacy`
(`legacy.go:121-127`) constructed a `State` by hand and returned it without the
call. `LegacyBridge` is itself a `StateStore` (`var _ StateStore =
(*LegacyBridge)(nil)`), so this was a `Load` path that skipped validation.

## Why it was not exploitable

Each individual check happened to be covered by something else:

- names are parsed by `ParseMigrationName` in the loop at `:110`
- `FormatVersion` is hardcoded to the current constant
- `Projection` is length-checked at `:104`, and malformed content is caught
downstream by `classifyAgainst` (`gate.go:158-166`), which correctly errors
rather than re-baselining

So no exploit was constructible. This is the defect `ValidateState`'s own
comment warns about — "validating at the point of use would leave each caller to
remember" — one refactor away from mattering, rather than a live hole.

## Severity

Minor. Defence-in-depth on a path with no demonstrated gap, but worth closing
precisely because the convention it breaks is the one that keeps the other three
backends honest.

## Resolution

`loadLegacy` now calls `ValidateState` on the constructed state before returning
it. A failure warns and returns `(nil, nil)`, joining the **single** deliberate
legacy fail-open rather than creating a second one: bootstrap is already where
an unreadable legacy marker lands, and that fallback is safe because this path
is only reached when the new store has nothing recorded.
