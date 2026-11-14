---
id: BUGA-7DDDXUI
type: bug-analysis-checklist
title: 'Analysis: List-context commands receive ungated, unredacted rows on stdin'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (with the PR #2247 fix reverted in a scratch edit, `TestCommandExec_ListContextIsGatedAndRedacted` reports the hidden `status` value and the fully hidden row in the payload, and `TestGoldenParity` fails on the `BUG-2TPO2F-command-list-input` recordings)
- [x] Minimal reproduction steps documented (run a `context: list` command as a principal whose ACL hides a row or a field; the script's stdin, echoed over SSE, carries it)
- [x] Environment/conditions noted (any build tag; any declarative ACL that hides rows or fields of the list's type; the principal needs only the command's `permission:`)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (read the payload through the same gated header read as `GET /api/v1/<list>`: `ReadQuery` plus `scopedEntities`, bodies for admitted rows only, then `redactEntity` per row; list `filters:` run on the redacted rows)
- [x] Regression test planned (`TestCommandExec_ListContextIsGatedAndRedacted` plus the re-recorded golden case)
- [x] Related areas checked for similar issues (entity and view command contexts already gated; settings relation targets moved onto the gated read in the same PR; remaining `scopedHeaders` callers tracked in TKT-D3RAB0E)
