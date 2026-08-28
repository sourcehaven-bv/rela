---
id: TKT-UFV01M
type: ticket
title: Review checklist must not track PR URL or CI status — they deadlock the done-before-PR gate
kind: enhancement
priority: medium
effort: xs
status: done
---

## Problem

The review-checklist template ends with a Pull Request section:

```markdown
## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
```

Two of those items — and the `**PR:**` field — record facts that **cannot exist
yet**. The `/pr` command gates on the ticket already being `done` and validating
clean, and `done-review-checklist-no-unchecked` (severity: error) rejects a
`done` review-checklist with any unchecked item.

So:

1. To open the PR, the checklist must be `done` with everything checked.
2. To check "PR URL documented", the PR must already exist.

That is a cycle. It was hit for real on TKT-MTWQ4G: validation failed with
`REV-7ILS94: Done review checklists cannot have unchecked items`, and the only
way through was to open the PR anyway, then backfill the URL in a follow-up
commit and burn a second CI cycle.

The workarounds are all bad. Checking the boxes before the PR exists means
asserting "All CI checks pass" when CI has not run — the checklist stops being
evidence and becomes a formality. Marking them skipped needs a reason that would
read "N/A: cannot be known yet", which is not a skip.

## Root cause

The checklist tracks facts that live **outside** the checklist's own timeline.
PR URL and CI status are properties of a PR that by construction post-dates the
ticket being done. GitHub already records both, authoritatively and
automatically — the PR page links its commits and shows its checks.

The other checklist items are all things a human decides and can verify before
opening a PR ("lint clean", "review-responses addressed"). These three are not.

A second, related contradiction sat in `CLAUDE.md`: the agent workflow listed
"**Create PR** (before `done`)" as step 4 and "**Complete** (status: `done`)" as
step 5, while `/pr` refuses to run until the ticket is already `done`. The
documented order was the reverse of the enforced one.

## Fix

Drop the two unknowable items and the `**PR:**` field from the template. Keep
one actionable item — running `/pr` — since that IS a pre-PR action. A comment
in the template records why the other two are absent, so nobody adds them back.

Swap the `CLAUDE.md` steps so the documented order matches the enforced one:
complete (`done`) first, then create the PR.

The commit message and branch already carry the ticket ID, so the ticket→PR link
is recoverable from git without duplicating it in the graph.

## Scope

- `tickets/templates/entities/review-checklist.md` — drop the two items and
the PR field; add a comment explaining the omission
- `CLAUDE.md` — reorder the workflow steps to match enforcement, and replace
"Document PR URL in review-checklist" with why that is not done

`.claude/commands/pr.md` needs no change: it only uses the PR URL for monitoring
and its own final report, never instructing that it be written into a checklist.

Existing done checklists are **not** rewritten: 171 of them carry the old
section with the items already checked, which still validates. The template only
governs newly created checklists.

## Out of scope

The `/pr` done-before-PR gate itself. It is correct — a PR should represent
finished work. The bug is that the checklist asked for something the gate makes
unknowable, not that the gate exists.
