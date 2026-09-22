---
id: AM-status-gates-agree
type: automated-measure
title: "The two ticket-status gates agree on every status, or say why they differ"
kind: ci
location: .github/workflows/ci.yml (Ticket done-before-merge check) + tickets/schema.yaml (ci-no-* rules)
status: active
description: "The mergeable-status policy is written twice — as ci-no-* rules in tickets/schema.yaml (corpus-wide) and as a case list in .github/workflows/ci.yml (diff-scoped). Nothing forces them to agree. Until a test derives one from the other, changing either requires checking the other and recording any deliberate divergence at the CI Gates block in schema.yaml."
---

Pins [[BUG-548IE9]].

## What to check

Editing the accept list in the "Ticket done-before-merge check" job, or adding
or removing a `ci-no-*` status rule, means checking the other file. For each
status in `ticket_status` / `bug_status`, the two must either agree, or the
divergence must be recorded with its reason.

Current state:

| status | job (diff-scoped) | rules (corpus-wide) | |
|---|---|---|---|
| `done`, `wont-fix`, `backlog` | accept | accept | agree |
| `blocked` | reject | reject | agree — corrected in BUG-548IE9 |
| `review`, `in-progress`, `analyzing`, `planning` | reject | reject | agree |
| `ready` | reject | accept | **deliberate** — see below |

## Why `ready` diverges on purpose

A `ready` item is legitimately parked in the repo; twelve exist on develop. But
a PR that *touches* one and merges its code has shipped a stale ticket, which
is how TKT-QXHFJZ shipped. Only the diff-scoped job can tell those apart, so
`ready` is enforced there and nowhere else.

A `ci-no-ready-*` rule would fail CI on every pre-existing ready item. Do not
add one.

## Why this is a human check, not a test

The real fix is P4 on BUG-548IE9: derive one list from the other, or fail a
test when they disagree on a status neither file marks as intentionally scoped.
That is more than the one-line correction the bug needed, so until it exists
this measure is a human check, and it is honest about being one.
