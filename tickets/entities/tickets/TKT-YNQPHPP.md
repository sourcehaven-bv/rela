---
id: TKT-YNQPHPP
type: ticket
title: 'Flows: M2 and M4b review leftovers'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split from TKT-FWQZGAM. Leftovers from the M2 code review (TKT-ZAXRXND) and the
M4 part B review (TKT-ZO9IB6V).

- `FlowControl.SubjectRuns` reads every run of a subject with no cap and
builds the detailed view for each. Cap it, report `truncated`, and drop the
detail from the list.
- `localHolders` caps the local-role query at `maxRoleHolders` without
setting `Truncated`. Narrow it to the candidate subjects instead.
- The `complete` affordance (`mayUpdate`) builds its own write request. Ask
the manager for a dry-run decision so the button and the POST cannot drift.
- A timer can move a claimed run before the step's write lands. `advance`
could defer to a live claim at the current generation.
- `useFlowList` reads a transient error as "no flows".
- `RunList` and `Tasks` take the subject from the `_carries` edge (bare
id); `Run` uses `run.Subject` with its face. Use one source.
- RR-JDRIMGP: the header status is set once at the fork. Refresh it as
branches move.
- RR-SP9FPDR: a join whose `when:` cannot be evaluated waits silently.
Show the failure in `Explain`.
- RR-5PQO65A: `ClosedBy` and `Region` clone every fork per call. Index by
join if profiling shows a cost.

## Acceptance

- Each item is fixed or closed with a reason.
