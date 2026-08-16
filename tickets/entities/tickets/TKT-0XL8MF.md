---
id: TKT-0XL8MF
type: ticket
title: 'Re-home affordance-resolver construction so appbuild can supply one: unblocks the field write gate and two read-side limitations'
kind: enhancement
priority: medium
effort: l
status: backlog
---

Originally scoped as "wire a policy-backed `FieldWriteGate`" (follow-up to
TKT-80EWGM). **Widened**: the blocker that stopped that ticket — `appbuild`
cannot construct an affordance resolver — has since been named as a KNOWN
LIMITATION at two further sites. The deliverable is the layering fix; the write
gate is one of its three consumers.

## The one missing capability, three symptoms

`appbuild` may not import `internal/affordances` under arch-lint (only
`dataentry` and `visibility` may), and the resolver is constructed inside
`internal/dataentry` (`affordances_stub.go:47`, `ResolverFromProfile`). Three
surfaces are degraded by exactly that:

| Site | Symptom |
|---|---|
| `entitymanager.FieldWriteGate` (`manager.go:220`) | Wired `AllowAllFieldGate{}` at both production sites — inert. MCP/Lua/CLI never field-gated. |
| `Services.ScheduledLuaWriteDeps` (`appbuild.go:349`, RR-7408F5) | Scheduled jobs get ROW gating only; `visible:` redaction does not apply. |
| `Services.GatedReads` (`appbuild.go:389`, TKT-UIR41P) | MCP read surfaces get ROW gating only; same gap. |

Fixing the layering once addresses all three. Fixing only the write gate leaves
two documented holes and a second future ticket to re-derive the same move.

Related: TKT-3FL2S6 (done) gated the analyze reader — the row-level half of the
same story. Its accepted residue is this field-level half.

## What to do

1. **Re-home construction.** Move the `affordances.New(...)` +
`statemachine.Compile` + `WithMachines` sequence out of `dataentry` into a
package both `appbuild` and `dataentry` may import — most likely a constructor
in `internal/affordances` itself, since it already owns `New`. `dataentry`
keeps `policyResolver` (its wire-shape adapter) and the
`AffordanceProfile` env handling, which are genuinely presentation concerns.
2. **Move `storeRelationLookup`** (`affordances_policy.go:88`) alongside it —
it is a pure `store.Store` → `RelationLookup` adapter with nothing
dataentry-specific in it, and every consumer needs one.
3. **Decide the arch-lint edge**: either add `affordances` to `appbuild`'s
`mayDependOn`, or introduce the shared constructor in a lower package so the
edge is unnecessary. Prefer whichever keeps `dataentry → affordances` intact
without widening `appbuild`'s 24-entry dep list more than needed.
4. **Implement `FieldWriteGate`** over `FieldVerdicts`, preserving denial
classification verbatim — **rule names are a wire contract**
(`affordances.go:264-272`), and the unknown-field → `RuleFieldHidden` mapping
closes the F8 side channel.
5. **Wire the read sites**: supply the resolver to `ScheduledLuaWriteDeps` and
`GatedReads`, and delete both KNOWN LIMITATION notes. Do not delete a note
without the corresponding test.
6. **CLI keeps `AllowAllFieldGate`** — the operator has full filesystem access
to the data, so gating there buys nothing (settled in TKT-80EWGM).
7. Consider migrating dataentry's PATCH onto `PatchEntity` at the same time, so
the gate has one implementation and one call site.

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

## Constraints discovered while widening

- **`WithMachines` is not concurrency-safe** (`resolver.go:171`). It is the one
mutator on an otherwise-immutable resolver and is safe today only because its
sole call site is synchronous wiring before the resolver escapes. Sharing one
resolver across more consumers must preserve that: construct fully, then
publish. Do not call it after the resolver is reachable from a request path.
- **`declarative.Policy()` is the single source for the policy** (RR-WTLD).
Reading the policy through any other channel invites mismatched-pair bugs.
- **Nil-declarative and no-affordance-grants must still select the Nop
resolver.** Both are normal configurations (NopACL, no `acl.yaml`), not errors.
- **`New` rejects nil arguments**; a construction failure must REFUSE, not
degrade to a permissive resolver — matching `GatedReads`' existing
`visibility.DenyReader` behaviour (RR-GKCZO5).

## This is not a regression fix

`AllowAllFieldGate` preserves today's exact behaviour; the two read sites are
documented as row-gated. Nothing is getting weaker — the seams simply are not
yet load-bearing. Treat this as closing three disclosed gaps, not fixing a live
vulnerability.

## Acceptance

- A request-scoped caller cannot set or unset a property their policy hides,
via MCP or Lua, not just via the dataentry HTTP path.
- Scheduled Lua jobs and MCP reads receive `visible:`-redacted properties; both
KNOWN LIMITATION notes are deleted, each with a test that fails without the fix.
- CLI can still patch any property.
- dataentry denial responses (rule names, status codes) are byte-identical to
today — existing affordance tests are the regression net.
- Exactly one resolver-construction path remains; `dataentry` no longer calls
`affordances.New` directly.
- The three TKT-80EWGM constraint tests pass unmodified.
- `just arch-lint` passes.
