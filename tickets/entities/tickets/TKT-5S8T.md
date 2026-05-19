---
id: TKT-5S8T
type: ticket
title: Clean tickets/ data debt that surfaces once conflict-marker detector is fixed
kind: chore
priority: medium
effort: m
status: backlog
---

## Context

Once BUG-???? (filed alongside this ticket) fixes the conflict-marker detector
to line-anchor its match, PLAN-ABRRT.md will load properly and rela's validator
will see ~40 pre-existing errors in `tickets/` data that have been silently
masked.

The masked errors fall into clean categories:

| Category | Count | Description |
|---|---|---|
| Tickets in `in-progress` / `review` | 8 | Already shipped via merged PRs (TKT-6WLSW #669, TKT-QETTR #673, TKT-4VLN #704, TKT-E6094 #716, TKT-GFQK #708, TKT-J5BET #688, TKT-PGK91 #668, TKT-ZEKO4 #698) — status just never transitioned to `done` |
| Planning checklists in-progress | 3 | PLAN-K49T, PLAN-KA7U, PLAN-W21Z — verify parent ticket status |
| Review checklists in-progress | 6 | REV-06ZH, REV-31JF, REV-AGS8, REV-P9FJ, REV-V5QER, REV-W2ZJ — verify parent ticket status |
| Done planning checklists with unchecked items | 10 | PLAN-HQ5Y, PLAN-HWQ6, PLAN-I3A8G, PLAN-JTLN, PLAN-MXQKI, PLAN-NRZ5, PLAN-V6BB, PLAN-VRXT, PLAN-WHOK, PLAN-XKMJ — strikethrough N/A items per CLAUDE.md convention |
| Done review checklists with unchecked items | 1 | REV-L5Z1 — same |
| Open review-responses | 12 | All on TKT-PGK91 (the git-crypt feature); transition to `addressed` with the merged PR ref, or `wont-fix` with reason |

## Scope

Mechanical sweep: cross-reference each entity against `git log` to find its
merged PR, then transition status. No new design judgment — every state is
provably stale.

## Acceptance criteria

1. `rela validate --check cardinality --check properties --check validations` on `tickets/` exits 0.
2. No ticket changed to `done` unless its implementing PR is verifiably merged on develop.
3. Each transitioned entity carries a short "auto-closed: see PR #XYZ" note in its body/resolution field.

## Out of scope

- New design or process changes.
- The conflict-detector fix itself (separate bug).
- Any ticket *not* in the categorized list above.

## References

- Blocks no specific PR (after the conflict-detector workaround in TKT-GN5LN's PR was reverted, the debt is hidden again). But every future PR that touches PLAN-ABRRT-like content will re-expose it.
