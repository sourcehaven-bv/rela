---
id: TKT-W76LRP
type: ticket
title: Ticket validation silently passed over unparseable entities; backfill the 10 violations that reached develop
kind: chore
priority: high
effort: s
status: done
---

## Problem

`rela validate` reported success on develop while 10 workflow-gate violations
sat in the graph. The Rela Tickets job was green on the PRs that introduced
them, so nothing flagged it until an unrelated PR happened to run validation
after the parse error was fixed.

### Root cause: a parse error silently shrinks the graph

An entity whose frontmatter does not parse is **skipped** by the loader, not
fatal to it. The store logs a warning and carries on:

```
WARN appbuild: failed to index entities error="... list errors:
  [failed to parse frontmatter: yaml: line 4: mapping values are not allowed ...]"
✓ All cardinality constraints satisfied
✓ All entity properties are valid
```

`validate` then checks a graph that is quietly missing entities and reports
success. Verified directly: with one bad file, validation prints **"All
validations passed"** while the guard added here fires.

This is visible in the CI log of #1338, which merged 7 of the 10 violations
with a green Rela Tickets job. Every workflow gate is only as strong as the
parser feeding it.

### Second hole: nothing validates after merge

The gate ran on `pull_request` only (`EVENT_NAME != "pull_request"` →
`applies=false`). A PR can only see its own diff, so a rule that starts failing
because of what already landed reaches develop unnoticed and then fails the
next unrelated PR — blaming the wrong change.

## Fix

**Guards (the durable part):**

- New `Entities all parse` step, ordered *before* validation, fails on any
  `failed to parse frontmatter` and explains the quoting fix. Validation
  results are not trusted until the parser is clean.
- The gate now also runs on push to `main`/`develop` (validation only; no
  `require_new`, since a push has no ticket to add). Post-merge breakage is now
  pinned to the merge that caused it.
- `Ticket done-before-merge check` scoped to `pull_request`, since it diffs
  against a PR base that does not exist on push.
- CI also triggers on `merge_group`, and the ticket gate validates there. This
  is a prerequisite for enabling a merge queue on develop: queue entries are
  tested on an ephemeral `gh-readonly-queue/...` ref, and a required check that
  never runs there leaves the entry queued forever. The gate treats a queue
  entry like a push (validate, but require no new ticket) — reporting a hollow
  success at the last gate before the commit lands would make the queue's
  guarantee worthless.

**Rule correctness:**

`ci-no-open-review-responses` failed any `status=open` review-response
unconditionally. That is wrong for a finding raised by `/design-review` on a
ticket still in the backlog: it is *supposed* to stay open until the ticket is
picked up, and failing CI pressures people into a resolved status that
misrepresents the state. It now evaluates the parent
(`open-review-response-parent-started.lua`): open is allowed only while every
parent work item is `backlog`/`ready`; an orphaned finding is still an error.

**Backfill (the 10 violations):**

| Entity | Violation | Resolution |
| --- | --- | --- |
| BUG-219PVU | no `has-review` | review checklist backfilled from #1338 |
| BUG-762I34 | no `has-review`, no `description` | checklist backfilled from #1340; description written from the bug's own body |
| BUG-ZE4354 | no `has-review` | review checklist backfilled from #1334 |
| TKT-UIR41P | no `has-review`, no `has-docs` | review checklist backfilled from #1338; **missing docs actually written** |
| RR-H8S10M, RR-P34E8J, RR-PQ5UN1 | open findings | rule corrected — parent TKT-BDG8U9 is backlog, so open is right |

The backfilled review checklists are explicitly labelled as reconstructed after
the fact and record only what their PRs actually did. They claim no new
verification.

The one exception is TKT-UIR41P's docs: `docs/mcp-server.md` had no mention of
the ACL-gated read seam part 1 introduced, so marking a docs checklist N/A would
have been false. The documentation was written here — a new "Read gating (ACL)"
section covering the `GraphReader` seam, row/field-level gating over MCP tools
and resources, the required principal, and an honest scope note that stdio MCP
is a local process rather than a network boundary.

## Verification

- `rela validate --check cardinality --check properties --check validations`
  passes on the tickets project (was 10 errors)
- Guard fires on an injected parse error, and validation demonstrably reports
  "All validations passed" for the same tree — the exact silent-pass reproduced
- New Lua rule verified in all three branches: parent backlog (passes), parent
  flipped to in-progress (fails, naming the parent), parent relation removed
  (fails as orphaned)
- `just docs` regenerated `docs/mcp-server.md` from the source guide; generated
  output matches the source edit
