---
id: IMPL-HLR62T
type: implementation-checklist
title: 'Implementation: Plumb AutomationName through automation.Result for per-action audit attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestProcess_TagsResultsWithAutomationName`
in `internal/automation`, pinning the engine half locally so a regression there
surfaces at the seam rather than three packages away as a wrong string in an
audit assertion.
- [x] Integration tests written (test full flow, not just units) — two new tests
in `internal/entitymanager/audit_test.go` driving a real `Manager.CreateEntity`
through the automation engine, the cascade runner and the audit sink.
- [x] Happy path implemented — name plumbed at the engine seam, ctx tagged per
entry in the runner.
- [x] Edge cases from planning handled — empty name, multi-automation, both
relation and entity paths.
- [x] ~~Error handling in place~~ (N/A: no new error paths; this adds a string
to an existing record and changes no control flow.)

## Test Quality

- [x] Using fixture builders or factories for test data — the engine test uses
the package's existing `newAutomation(...).OnCreate(...).Build()` fluent builder
and `testutil.Entity`, matching the file's convention rather than hand-rolling
literals.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test — each automation declares
exactly the one action its assertion covers.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*Both new behaviours were verified by MUTATION, not just by a green test* — for
an attribution change, a passing assertion proves little unless the assertion is
known to fail when the attribution is wrong.

**AC3 — tightened test actually catches the regression.** Reverting the two
`AutomationName:` assignments in `engine.go` and re-running:

```
audit_test.go:473: cascaded create-entity: want TriggeredBy="automation:create-checklist-for-req", got "automation"
audit_test.go:478: cascaded create-relation: want TriggeredBy="automation:create-checklist-for-req", got "automation"
```

Exactly the generic label the issue reports. Restored → passes.

**AC1/AC2 — and the test that first passed for the wrong reason.** The
discriminating test uses **two** differently-named automations, because with one
an implementation that hoists the ctx tag out of the per-entry loop would pass
anyway. The first version of this test did *not* catch that: mutating the runner
to hoist the tag left it green.

The reason was worth finding — `spawn-checklist` emitted a `create_entity` and
`link-decision` a `create_relation`, so the two never shared a
`RelationsToCreate` slice and there was nothing to mis-order. `Engine.Process`
aggregates **every matching automation into one `Result`**
(`engine.go:238-246`), so a mixed slice genuinely occurs — the test just wasn't
producing one. Reworked so both automations emit relations, and re-ran the
mutation:

```
audit_test.go:745: informed-by relation: want automation:spawn-checklist, got "automation:link-decision"
```

Now it discriminates. This is the bug the per-entry wrap exists to prevent, and
without the mutation check the test would have been decorative.

**AC4 — fallback preserved. My first claim here was WRONG, and code review
caught it.**

The empty-name half is fine: `TestAudit_UnnamedAutomationKeepsGenericLabel`
drives an automation with an empty `Name:` and asserts the record says
`automation`, not a dangling `automation:`.

The **pre-wrapped-ctx** half was not. I credited
`TestAudit_IfExistsReplaceUsesCascadeLabel`, but that test asserts a label
`cascadeHost.DeleteEntity` stamps *internally*, downstream of the runner's wrap
— so it never exercises an enclosing label at all. The negative test I was
relying on tested a different code path than I believed, which is worse than
having no test because it bought false confidence.

Behind that gap was a real regression this ticket introduced (RR-ZYVERL):
`audit.WithTriggeredBy` is an unconditional `context.WithValue`, so the new
per-entry tag **overwrote** an enclosing label. Measured before and after on the
production path, with a scheduler ctx pre-stamped `schedule:nightly`:

```text
BEFORE                                     AFTER (the regression)
create-entity   requirement   schedule:nightly   schedule:nightly
create-entity   checklist     schedule:nightly   automation:spawn-checklist
create-relation has-checklist schedule:nightly   automation:spawn-checklist
```

"What did last night's task write?" silently returned fewer rows. Nothing
errored; no test failed.

Fixed by making `triggeredByCtx` compose rather than clobber — it declines when
the ctx already carries a label, mirroring `recordCascade`'s own guard. The
policy is now a written decision rather than an accident: **the outermost cause
wins**, because `triggered_by` is a single string and the enclosing cause is the
operator-facing one. Documented in the helper and in the audit-log guide. Pinned
by `TestAudit_OuterLabelSurvivesCascade`, verified to fail with
`got "automation:spawn-checklist"` when the guard is removed.

**AC4b — `if_exists: replace` consistency (RR-4JUX43).** Review also found the
replace-delete carried the generic label while its matching create carried the
specific one — two labels on adjacent rows of one operation, so a filter on the
automation's name returned the create and missed the delete. Caused by deriving
the tagged ctx *after* `handleIfExists`. Hoisted above it; verified the delete
now reads `automation:replace-checklist-on-active` while the cascaded
relation-deletes still read `cascade:delete-entity:CL-001`. Pinned by
`TestAudit_IfExistsReplaceAttributesDeleteToTheAutomation`.

**AC5 — docs.** The known-gap section is removed from the guide entity and
`docs/audit-log.md` regenerated via `./scripts/generate-docs.sh` (the file is
generated — *"auto-generated from docs-project/entities/. Do not edit
directly."*). The regeneration touched **only** `docs/audit-log.md`, confirming
no unrelated drift. The `triggered_by` reference list was also corrected: it
previously described `automation:<name>` as scripted-only and `automation` as
"generic label by design", both now false.

**Full-suite and gates:** `go test ./...` exit 0; builds under all four backend
tags; `just lint` 0 issues; `comment-lint`, `arch-lint`, `plimsoll`, `lint-md`
all clean.

## Quality

- [x] Code follows project patterns — this generalizes the mechanism the
*scripted* path already uses, three lines from where the change lands:
`LuaToExecute.AutomationName` (`types.go`) set at the same engine seam, and
`audit.WithTriggeredBy(ctx, "automation:"+name)` in `executeScriptActions`
(`runner.go`). The audit layer needed no change at all — `recordCascade` already
prefers a ctx label and only falls back to the generic one.
- [x] Checked for DRY opportunities — the ctx tag is a single `triggeredByCtx`
helper shared by **all three** cascade write paths: relation creations, entity
creations, and scripted actions. The scripted path was NOT in my first pass
(review caught it, RR-KPTYPY); it had been stamping the label inline and
unguarded since long before this ticket, which is where both the dangling
`automation:` and the scheduler-label clobbering originated. Ordering mattered:
routing it through the helper was only safe *after* the helper itself was fixed
to compose — landing it first would have spread the clobbering bug to a fourth
call site.
- [x] No security issues introduced — this touches the audit log, so two
properties were checked explicitly: attribution only becomes *more* specific (no
record loses a field, and the generic fallback survives), and the name is
operator-authored config from `schema.yaml`, which CLAUDE.md states is not
secret. Principal attribution is untouched.
- [x] No silent failures — the one silent-failure risk was the empty-name case
emitting a dangling `automation:`; `triggeredByCtx` returns ctx unchanged
instead, and that is tested.
- [x] No debug code left behind — the temporary record-dumping test used to
diagnose the metamodel direction issue was removed.

**One design note worth carrying to review.** `RelationsToCreate` changed from
`[]*entity.Relation` to `[]RelationToCreate` rather than gaining a parallel
`[]string` of names. A parallel slice can drift in length or order with no
compile-time signal, and this value crosses a package boundary into
`autocascade` where the consequence is a silently misattributed audit record.
The struct makes the pairing unrepresentable-if-wrong; the cost is three
mechanical test-site updates.
