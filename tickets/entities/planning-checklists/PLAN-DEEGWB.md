---
id: PLAN-DEEGWB
type: planning-checklist
title: 'Planning: Review checklist must not track PR URL or CI status — they deadlock the done-before-PR gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the review-checklist template's Pull Request section, and the `CLAUDE.md`
workflow steps that describe it.

OUT: the `/pr` done-before-PR gate (correct as-is), the
`done-review-checklist-no-unchecked` validation rule (also correct — it is what
surfaced the contradiction), and the 171 existing done checklists that carry the
old section.

**Acceptance Criteria:**

1. A newly created review-checklist can reach `status=done` and validate clean
with no PR in existence. Test: this ticket's own REV, closed before its PR is
opened.
2. `rela validate --project tickets` stays clean over the 171 existing
checklists that still carry the old section.
3. `CLAUDE.md`'s documented step order matches what `/pr` enforces.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: `xs`, a template edit)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

Prior art in-repo: **TKT-IVQKQ3** ("Ticket gate rejects code-only and
non-bug-entity PRs") is the same class of bug — a workflow gate that rejected
legitimate shapes — fixed by narrowing what the gate asks for rather than by
removing the gate. Same feature (`FEAT-GE1YY`), same concept (`ci-pipeline`),
and the same principle applies here.

Checked every other checklist template for the same defect. `planning-`,
`implementation-`, `docs-` and `bug-analysis-checklist` track only facts
knowable at the time they are completed. The review-checklist was the sole
offender.

Grepped for other places instructing that the PR URL be recorded:
`CLAUDE.md:730` (fixed) and two references in `.claude/commands/pr.md` which are
monitoring/reporting only and correctly need no change.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Delete the two unknowable items and the `**PR:**` field. Keep "Run `/pr`", which
is a genuine pre-PR action. Add an HTML comment in their place recording why
they are absent — without it the next person reads the omission as an oversight
and adds them back.

**Alternatives rejected:**

1. *Relax `done-review-checklist-no-unchecked` to allow unchecked items in the
PR section.* Rejected: weakens a rule that is doing its job, and leaves items
that can only ever be filled in after the fact.
2. *Let the items be skipped with a reason.* Rejected: a skip means "does not
apply to this ticket". "Cannot be known yet" applies to every ticket, so it is a
template defect, not a per-ticket exemption.
3. *Move the PR URL to a ticket property set by `/pr` after opening.* Rejected
as out of scope and still redundant — GitHub is the source of truth and the
branch carries the ticket ID.
4. *Rewrite the 171 existing checklists.* Rejected: they validate, their items
are truthfully checked, and a mass edit would churn history for no gain.

**Files to modify:** `tickets/templates/entities/review-checklist.md`,
`CLAUDE.md`.

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: no code, no inputs)
- [x] ~~Input validation approach defined~~ (N/A)
- [x] ~~Security-sensitive operations identified~~ (N/A)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A)

**Input Sources & Validation:** N/A — documentation and a markdown template. No
executable code, no runtime surface.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

The change is exercised by this ticket itself: its review-checklist is created
from the new template and must reach `done` and validate clean *before* its PR
exists — which is precisely the case that failed on TKT-MTWQ4G. That is a real
end-to-end test, not a simulation.

Plus `rela validate --project tickets --check cardinality --check properties
--check validations` over the whole tree, covering the 171 old checklists.

**Edge Cases:**

- Old checklists with the removed items still present and checked — must keep
validating (they do; the rule reads content, not the template).
- A checklist created from the new template with "Run `/pr`" left unchecked —
must still fail. The rule is unchanged, so this is preserved.

**Negative Tests:**

Setting the new REV to `done` with its one remaining PR item unchecked must fail
validation, proving the rule still bites and this change only removed the
*unknowable* items, not the enforcement.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — `xs`

**Risks:**

1. *Losing the ticket→PR link.* Low. The branch name and every commit carry
the ticket ID, and GitHub indexes both; the link is recoverable by search. The
graph never was the authoritative record.
2. *Someone re-adds the items later.* The template comment explains why they
are gone and cites this ticket, which is the mitigation.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `CLAUDE.md` — workflow step order and the PR-URL instruction
- [x] N/A for `docs/` and `README.md` — contributor workflow, not a
user-facing rela feature

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: `xs`
template edit with a single, already-diagnosed cause; the design question —
remove the items vs. relax the rule — is recorded under Alternatives above.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
