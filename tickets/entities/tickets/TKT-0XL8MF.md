---
id: TKT-0XL8MF
type: ticket
title: 'Wire a policy-backed FieldWriteGate so MCP and Lua callers cannot write fields their policy hides'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Follow-up to TKT-80EWGM, which introduced `entitymanager.FieldWriteGate` as a
required injected capability but wires `AllowAllFieldGate{}` at **every**
production site, leaving it inert.

The blocker that deferred it — `appbuild` could not construct an affordance
resolver — **is now resolved**. [[TKT-BUYEW1]] added the `appbuild ->
affordances` arch-lint edge and `buildFieldRedactor`, which builds a fully
initialized resolver once during assembly. This ticket consumes that.

## Scope

This is the **write** half. The read half (field-level `visible:` redaction on
`ScheduledLuaWriteDeps`, `GatedReads`, and the automation cascade read deps)
shipped in [[TKT-BUYEW1]].

## Not a regression

`AllowAllFieldGate` preserves today's exact behaviour. Field-level write gating
has only ever existed on the dataentry HTTP path, which calls
`validateFieldWrite` directly and is unchanged. MCP, CLI and Lua were never
field-gated. Nothing got weaker; the seam simply is not yet load-bearing.

## What to do

1. Implement `FieldWriteGate` over `affordances.FieldVerdicts`, preserving the
denial classification verbatim — **rule names are a wire contract**
(`affordances.go:264-272`), and the unknown-field → `RuleFieldHidden` mapping
closes the F8 side channel.
2. Wire it in `appbuild` for request-scoped surfaces. `buildFieldRedactor`
currently discards the `*affordances.PolicyResolver` after wrapping it in a
`visibility.PolicyRedactor`; the gate needs the resolver itself, so hold it
alongside rather than building a second one (`WithMachines` makes a second
construction a concurrency hazard as well as waste).
3. **CLI keeps `AllowAllFieldGate`** — the operator has full filesystem access
to the data, so gating there buys nothing (settled in TKT-80EWGM).
4. Consider migrating dataentry's PATCH onto `PatchEntity` at the same time, so
the gate has exactly one implementation and one call site. That would also let
`dataentry` drop its own `affordances.New` call in favour of the shared one,
and retire the duplicated `storeRelationLookup` (see below).

## Constraints inherited from TKT-80EWGM (do not re-litigate)

- **The gate runs AFTER `authorizeAndAudit`, never before** (RR-32XA5V). Field
verdicts are value-dependent, so consulting them for an unauthorized caller
leaks entity existence and stored values. Pinned by
`TestPatchEntity_GateRunsAfterAuthorize`.
- **Elevation is total** — `bypassACL` skips the field gate too (RR-BA1NIV).
Pinned by `TestPatchEntity_ElevationSkipsFieldGate`.
- **Automation output is not gated** (RR-00ERM9) — the gate constrains
caller-authored changes only. Pinned by
`TestPatchEntity_AutomationNotFieldGated`.

## Constraints carried over from the resolver work

- **`WithMachines` is not concurrency-safe** (`resolver.go:171`). It is the one
mutator on an otherwise-immutable resolver, safe only because its call site is
synchronous wiring before the resolver escapes. Construct fully, then publish.
- **`declarative.Policy()` is the single source for the policy** (RR-WTLD).
- **Nil-declarative and no-affordance-grants must select the permissive path** —
both are normal configurations, not errors.
- **A construction failure must REFUSE, not degrade** to a permissive gate.

## Known debt this ticket can retire

`storeRelationLookup` exists twice — `internal/dataentry/affordances_policy.go`
and `internal/appbuild/relationlookup.go` — because appbuild cannot import
dataentry. Both feed affordance `when:` predicates, so a bug fixed in one and
missed in the other makes two surfaces disagree about who may see what. Each
copy cross-references the other; hoisting into `internal/affordances` is the
real fix. Note both swallow store iteration errors and return "no edge", which
is fail-OPEN for a `when: not has_relation(...)` predicate — worth addressing
while consolidating.

## Acceptance

- A request-scoped caller cannot set or unset a property their policy hides,
via MCP or Lua, not just via the dataentry HTTP path.
- CLI can still patch any property.
- dataentry denial responses (rule names, status codes) are byte-identical to
today — existing affordance tests are the regression net.
- The three constraint tests above pass unmodified.
- `just arch-lint` passes.
