---
id: TKT-0XL8MF
type: ticket
title: 'Wire a policy-backed FieldWriteGate: move affordance-resolver construction out of dataentry so appbuild can supply it'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Follow-up to TKT-80EWGM, which introduced `entitymanager.FieldWriteGate` as a
required injected capability but wires `AllowAllFieldGate{}` at **every** site.

## Why it was deferred

The real implementation would adapt `affordances.PolicyResolver.FieldVerdicts` —
the same logic `dataentry.affordanceService.validateFieldWrite`
(`affordances.go:326`) already uses. But the resolver is constructed inside
`internal/dataentry` (`affordances_stub.go:75`), and `appbuild` may not import
`internal/affordances` under arch-lint (`.go-arch-lint.yml:410-434`). Supplying
a policy-backed gate means moving resolver construction to a package both
`appbuild` and `dataentry` may depend on.

That is a wiring/layering change with its own design, so TKT-80EWGM stopped at
the seam rather than widening its own scope.

## Important: this is not a regression

`AllowAllFieldGate` **preserves today's exact behaviour**. Field-level write
gating has only ever existed on the dataentry HTTP path, which still calls
`validateFieldWrite` directly and is unchanged. MCP, CLI and Lua were never
field-gated. Nothing got weaker; the seam simply is not yet load-bearing.

## What to do

1. Move (or re-home) affordance-resolver construction so a non-dataentry
wiring site can build one — likely a small constructor in
`internal/affordances`, with `dataentry` keeping only its wire-shape adapter.
2. Implement `FieldWriteGate` over `FieldVerdicts`, preserving the denial
classification verbatim — **rule names are a wire contract**
(`affordances.go:264-272`), and the unknown-field→`RuleFieldHidden` mapping
closes the F8 side channel.
3. Wire it in `appbuild` for request-scoped surfaces. **CLI keeps
`AllowAllFieldGate`** — the operator has full filesystem access to the data, so
gating there buys nothing (settled in TKT-80EWGM).
4. Consider migrating dataentry's PATCH onto `PatchEntity` at the same time, so
the gate has exactly one implementation and one call site.

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

## Acceptance

- A request-scoped caller cannot set or unset a property their policy hides,
via MCP or Lua, not just via the dataentry HTTP path.
- CLI can still patch any property.
- dataentry denial responses (rule names, status codes) are byte-identical to
today — existing affordance tests are the regression net.
- The three constraint tests above still pass unmodified.
