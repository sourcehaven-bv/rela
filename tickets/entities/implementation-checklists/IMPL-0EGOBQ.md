---
id: IMPL-0EGOBQ
type: implementation-checklist
title: 'Implementation: Lua scripts cannot distinguish an ACL-redacted property from a genuinely-unset one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Changes:**

| File | Change |
|---|---|
| `internal/entity/entity.go` | `Redacted []string` + `IsRedacted()`; `Clone` deep-copies; corrected the `InaccessibleReason` godoc that invited the rejected design |
| `internal/visibility/policyreader.go` | `Redact` records withheld names, freshly allocated + sorted |
| `internal/lua/runtime.go` | `EntityToTable` exposes `redacted` set; `luaEntityIsRedacted` |
| `internal/store/memstore/memstore.go` | clears `Redacted` on create/update (bug found by the new conformance test) |
| `internal/store/storetest/entity.go` | `Entity/RedactedNotPersisted` — runs against every backend |
| `docs-project/entities/guides/GUIDE-lua-scripting.md` | docs source; `docs/lua-scripting.md` regenerated via `just docs` |

No new error paths: `Redact` has no error channel, and the redactor contract is
total (returns a set, never fails). Nothing is swallowed.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reused existing fixtures rather than inventing new ones: `newACLWorld`
(`internal/lua/aclreads_test.go`) supplies a real `acl.Declarative` +
`affordances` resolver over memstore, and `newTestAppV1` + `mustNewACL`
(`internal/dataentry`) supply the real data-entry wiring. Tests assert against
the seeded entity, not against literals copied from it.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the feature three ways against a scratch project (`/tmp/redact-demo`) with a
real `acl.yaml` granting `visible: [title]` on `person`:

1. **CLI** (`rela script`) — `salary value = 100000`,
`salary redacted = false`. Correct: ungated runtime, no policy evaluated,
documented as such.
2. **Scheduler** (`run_as: alice`) — row gate applied (denied entirely
until the principal was granted read), but once readable: `salary=100000`,
`redacted_set=[]`. **Field redaction does not apply on this path** —
pre-existing RR-7408F5, filed as RR-IHWEB0 and documented in the runtime table.
3. **Data-entry** — the path that actually has a field resolver.
Verified by `TestACLScript_RedactedVisibleToLua`
(`internal/dataentry/acl_views_test.go`), which drives
`App.scriptReader(appRedactor(app))` exactly as document rendering does: hidden
value absent, `IsRedacted("status")` true, `IsRedacted("title")` false,
`IsLocked()` false.

Per-AC status:

| AC | Verified by | Result |
|----|-------------|--------|
| 1 — redacted distinguishable from unset | `TestScriptReads_RedactedIsDistinguishableFromUnset` | PASS — both read `nil`, only the hidden one reports redacted |
| 2 — value still unreachable | `TestScriptReads_RedactedNeverCarriesValues` | PASS — `100000` appears nowhere via `redacted`, `properties`, or `prop()` |
| 3 — nothing-hidden unchanged | `TestRedact_NothingHiddenIsUnchanged` | PASS — original face returned, no marker |
| 4 — validator does not skip | `TestRedactedDoesNotLock` + `TestRedact_DoesNotLock` | PASS — `IsLocked()` false, so `validator.go:198` cannot skip it |
| 5 — no bogus git-crypt 422 | same `IsLocked()` invariant (`write_handler.go:335` keys on it) | PASS |

Edge cases verified: all-properties-hidden (`TestRedact_AllPropertiesHidden`),
git-crypt AND ACL-redacted together (`TestRedact_PreservesInaccessible`), list
path vs get path (`TestScriptReads_ListEntitiesMarksRedacted`), ungated runtime
(`TestScriptReads_UngatedRuntimeReportsNothingRedacted`), repeated-call aliasing
(`TestRedact_RepeatedCallsDoNotAlias`), input not mutated
(`TestRedact_DoesNotMutateInput`).

**A real bug was found here, not by review-by-reading.** RR-KBWJPV asked for the
non-persistence assertion to live in `storetest` (every backend) rather than
markdown-only. Added, and **memstore failed while fsstore passed**: memstore
persists via `e.Clone()`, which had just been taught to deep-copy `Redacted`.
Fixed at memstore's write boundary. The markdown-only test originally planned
would have passed and the bug would have shipped.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `IsRedacted` mirrors `IsInaccessible`;
`TestCloneRedactedIsolation` mirrors `TestCloneInaccessibleIsolation`; the
sorted-names shape mirrors `redactedPropertyNames` (DEC-T0XIWQ).

DRY: deliberately did NOT extract a shared helper between
`redactedPropertyNames` (wire, computed from `FieldVerdicts`) and `Redact`'s
population (domain, computed from the `hidden` set). They consume different
inputs in different packages; a shared helper would couple `visibility` to
`dataentry`'s verdict type for three lines.

Security: the change widens disclosure by design (names, never values) —
justified in the ticket and pinned by tests asserting values never surface,
row-gating is untouched, and `WritePrepStore` reads stay raw.

Gates: `go test ./...` clean, `just lint` 0 issues, `just arch-lint` OK, `just
plimsoll` OK, `just coverage-check` PASS (76.9%).
