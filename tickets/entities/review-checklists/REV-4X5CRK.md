---
id: REV-4X5CRK
type: review-checklist
title: 'Review: Plumb AutomationName through automation.Result for per-action audit attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` **exit 0**, re-run after
every review fix. Builds under all four backend tags.
- [x] Lint clean (`just lint`) — **0 issues** (caught one `misspell`).
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links.
- [x] Coverage maintained (`just coverage-check`) — both thresholds PASS,
78.5% total.
- [x] `just arch-lint`, `just plimsoll`, `just lint-md` — all clean.

**Comment findings.** `just comment-report` introduced no new advisory finding
from this diff, and no suppression was added anywhere.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 9 findings — 1 critical, 5 significant, 2 minor, plus one
out-of-scope bug. **7 addressed, 2 deferred with reasons.**

RR-ZYVERL (critical), RR-4JUX43, RR-KPTYPY, RR-KA9LD5, RR-3DWRZX, RR-Z1V5Z6
(significant), RR-14X83F (minor) — addressed. RR-OPBDAY (minor), RR-2FY0O8
(significant) — deferred, see below.

**The critical finding was a regression I introduced (RR-ZYVERL), and my own
checklist claimed a test covered it that did not.**

`audit.WithTriggeredBy` is an unconditional `context.WithValue`. The new
per-entry tag therefore **overwrote** an enclosing label. Confirmed by running
the probe against the pre-change tree — this is a before/after regression, not a
pre-existing gap:

```text
BEFORE                                     AFTER
create-entity   requirement   schedule:nightly   schedule:nightly
create-entity   checklist     schedule:nightly   automation:spawn-checklist
create-relation has-checklist schedule:nightly   automation:spawn-checklist
```

"What did last night's scheduled task write?" silently returned fewer rows. No
error, no failing test — the forensic-regression shape that only surfaces when
someone needs the log.

It slipped through because PLAN-VKUSB7 listed a negative test for exactly this
("pre-wrapped ctx → preserved label") and IMPL-HLR62T credited
`TestAudit_IfExistsReplaceUsesCascadeLabel`. That test asserts a label
`cascadeHost.DeleteEntity` stamps *internally*, downstream of the runner's wrap,
so it never exercises an enclosing label. **The negative test I relied on tested
a different code path than I believed** — worse than no test, because it bought
false confidence. The implementation checklist has been corrected to say so.

Fixed by making `triggeredByCtx` decline when the ctx already carries a label,
mirroring `recordCascade`'s own guard. The composition policy is now a **written
decision** rather than an accident: *the outermost cause wins*, because
`triggered_by` is a single string (not a stack) and the enclosing cause is the
operator-facing one. Recorded in the helper's godoc and in the audit-log guide.

**Two more attribution holes review found, both now closed:**

- **RR-4JUX43** — `if_exists: replace` emitted the generic label on its *delete*
half while the matching create carried the specific one, because the tagged ctx
was derived after `handleIfExists`. Two labels on adjacent rows of one
operation, so a filter on the automation's name returned the create and missed
the delete — the very question this ticket exists to answer. Hoisted the
derivation; the cascaded relation-deletes still keep
`cascade:delete-entity:<id>`.
- **RR-KPTYPY** — the pre-existing scripted path stamped `"automation:"+name`
inline and unguarded, so a nameless automation wrote a literal dangling
`automation:`, and it clobbered enclosing labels the same way. Now routed
through the shared helper. **Ordering was load-bearing**: doing this before
fixing the helper would have spread the clobbering bug to a fourth call site.

**Docs correction (RR-Z1V5Z6).** The sentence I added — "you should not normally
see it, since every automation declared in `schema.yaml` has a `name:`" — was
false twice over. `name:` is an ordinary optional field on an `automations:`
list entry with **no loader validation** behind it, and the `if_exists: replace`
hole meant the bare label appeared routinely. Both halves rewritten; documenting
an unenforced invariant as a guarantee would have sent the next operator on a
false-positive hunt.

**Deferred, with reasons:**

- **RR-2FY0O8** — automations in an `includes:` file are **silently dropped**
(`partialMetamodel` has no `Automations` field: not merged, and not in the "not
allowed in included files" list that errors, so it falls through the gap).
Verified empirically: the include is read, `len(meta.Automations)` is 0, no
warning. Genuinely out of scope — a metamodel-include bug with its own blast
radius — so it is filed as **BUG-IK9IYL** rather than folded in here.
- **RR-OPBDAY** — cosmetic `automation ""` / `automations.` diagnostics. The real
remedy is making an empty name impossible at load time, which belongs with the
validation work, not with an attribution ticket.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — per-automation relation attribution. PASS.**
`TestAudit_PerAutomationAttribution` uses two differently-named automations both
emitting relations into one `Result`. Mutation-verified: hoisting the ctx tag
out of the loop fails it with a cross-attributed name.
- **AC2 — same for entities. PASS.** Same test, plus
`TestAudit_NestedCascadeAttribution` for the multi-level case (an entity created
by automation A that itself triggers B — B's write names B).
- **AC3 — tightened existing test. PASS.** Mutation-verified: reverting the
engine assignments fails it with `got "automation"`.
- **AC4 — fallback and enclosing labels. PASS**, after the RR-ZYVERL fix.
`TestAudit_UnnamedAutomationKeepsGenericLabel` (empty name → generic label, not
a dangling colon) and `TestAudit_OuterLabelSurvivesCascade` (enclosing label
wins), the latter mutation-verified.
- **AC5 — docs. PASS.** Gap section removed from the guide entity,
`docs/audit-log.md` regenerated (generated file — never edited directly);
regeneration touched only that file. `triggered_by` reference list corrected,
and the one-label-per-record policy documented.

**Every behavioural claim in this ticket was verified by mutation**, not just by
a green test — for an attribution change a passing assertion proves little
unless the assertion is known to fail when attribution is wrong. That practice
is what exposed the first version of `TestAudit_PerAutomationAttribution`
passing for the wrong reason (both automations wrote to different slices, so
nothing could be mis-ordered), and it is why the reworked test is trustworthy.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` — see below.
- [x] User-facing documentation updated — `docs-project/entities/guides/GUIDE-audit-log.md`
(source) → `docs/audit-log.md` (generated).
- [x] Docs-checklist marked as done

**Docs Checklist:** covered inline — the doc change is two edits to one guide
entity (remove the stale known-gap section; correct the `triggered_by` label
list and add the composition policy), both verified in the regenerated output.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — none added.
- [x] Ready for another developer to use — `triggeredByCtx`'s godoc states both
guards and *why* each is load-bearing, so the next person does not reintroduce
either. The audit-log guide documents the label vocabulary and the
outermost-wins policy in one place.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design, not
skipped: `/pr` gates on the ticket already being `done` and validating clean,
and a `done` checklist may have no unchecked items — so this item can only be
satisfied by a PR that does not exist yet. Same ordering the note below records
for the PR URL and CI status.)
