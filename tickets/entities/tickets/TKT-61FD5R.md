---
id: TKT-61FD5R
type: ticket
title: Record the archplan baseline and run archplan in CI
kind: chore
priority: medium
effort: m
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

Record a reviewed archplan baseline and run archplan in CI, so new architecture
and security findings show up on pull requests.

- Pin archplan v26.10.30 and its commit in the justfile. v26.10.29 lacks the
misplaced_file and unrelated_parts detectors that the baseline proposal
reviewed, so its finding set differs.
- Fix two findings by tuning instead of accepting them: guard markers on the
comment gate helpers (with the comment:read check moved into a marked
`listGate`), and `gated_by: [visibility]` for `rollup`.
- Record `.archplan/baseline.json` with one reason per finding group. The 5
SQLite FTS shadow tables (`entity_search_*`) stay out: their `data.orphan_table`
findings are an archplan regression.
- Add an `Archplan` job to ci.yml on `pull_request` and `merge_group`. It
exports the merge base in its own worktree (with `npm ci`) and runs `archplan ci
--base-model` on the static model. `archplan ci --base` cannot be used: its
temporary worktree has no `frontend/node_modules`.
- Add a weekly workflow that runs the seqtrace routes suite and the
trace-based checks, and keeps one tracking issue for new security findings and
worse ratchet metrics.

## Acceptance criteria

- The Archplan job passes on this PR.
- The baseline has no generic "baseline" reasons.
- `handleV1Comments` is no longer `unchecked_entry`, and rollup is no longer
`unguarded_path`.
- The weekly workflow updates one issue instead of opening duplicates, and
never publishes finding details outside code scanning.
