---
id: TKT-IVQKQ3
type: ticket
title: Ticket gate rejects code-only and non-bug-entity PRs
kind: enhancement
priority: medium
effort: s
description: 'The "Rela Tickets" CI gate required a changed file under one of three entity directories (bugs/features/tickets), which rejected two legitimate PR shapes: a ticket-hygiene PR touching only review-responses or doc-tasks, and a code-only PR whose tickets were deliberately split onto a companion branch.'
status: done
---

## Description

The `Check for new or modified bug/feature/ticket entity` step in `ci.yml`
globbed exactly three of the fifteen directories under `tickets/entities/`:
`bugs/`, `features/`, `tickets/`. Anything else counted as "no ticket".

Two real PRs failed on this without doing anything wrong:

- **#1209** flips `RR-UGKOI5` from `deferred` to `addressed` and closes out
  `DOC-WWNE41`. Both are work-item entities and this is precisely the ticket
  hygiene the gate wants — it failed for touching the "wrong" subdirectory.
- **#1212** is the `_views` ACL redaction fix, deliberately kept code-only so
  it is reviewable as code, with its tickets on companion branch #1214. The
  gate structurally cannot pass for a clean code/ticket split.

The failure mode this creates is worse than the gap: the cheapest way to get
green is to bolt a no-op edit onto an unrelated ticket, which is exactly the
stale-ticket problem the gate exists to prevent.

## Fix

Widen the allowlist to every *work-item* entity type — bugs, features,
tickets, research, review-responses, doc-tasks and the five checklist types.
Reference material (`concepts/`, `decisions/`, `ideas/`, `future-concepts/`,
`automated-measures/`) stays out: editing a concept is not evidence of
tracked work.

For the code-only split, a PR may declare its companion in the body:

```text
Tickets-PR: #1214
```

Ticket state is then gated on that PR, which carries the entities and must
itself pass the done-before-merge check before it can merge.

## Scope

The `Ticket done-before-merge check` step is **unchanged** — it still scans
only `bugs/` and `tickets/` and still requires a terminal status. Widening
what counts as *referencing* the ticket graph does not widen what counts as
*finishing* the work, so no unfinished bug becomes mergeable as a result.

The PR body is read via an `env:` binding and only `[0-9]+` is ever extracted
from it, so the trailer is not a script-injection surface.
