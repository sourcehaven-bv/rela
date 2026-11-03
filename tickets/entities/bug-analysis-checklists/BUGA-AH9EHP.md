---
id: BUGA-AH9EHP
type: bug-analysis-checklist
title: 'Analysis: Gaining a role can narrow field visibility and writability'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`go test ./internal/affordances -run TestRoleUnion_ConferredRoleNeverNarrows` fails: admin watching TKT-1 gets hidden=[secret], read-only=[status secret], denied options=[review done])
- [x] Minimal reproduction steps documented (policy: admin `read/update: ["*"]` with no blocks; `watcher` conferred by a `watches` edge declares `visible:`, `fields:` and `options:` for ticket; alice holds admin and watches TKT-1; FieldVerdicts for alice on TKT-1 denies what watcher does not list)
- [x] Environment/conditions noted (any backend; needs a conferred or global role with a block plus a block-less role holding the verb, on the same row)

## Root Cause

- [x] Immediate cause identified (why1: FieldVerdicts skips a block-less role, so it adds nothing to `allowed` while another role's block closes the dimension)
- [x] Contributing factors found (why2-3: a missing block means "no opinion" in `dimension`; the resolver never consults the verb lists, so it cannot tell an admin from a viewer)
- [x] Systemic cause explored (why4-5: no monotonicity test; the union rule is stated for verbs only, and each field dimension implemented its own fold)

## Fix Planning

- [x] Fix approach determined (a block-less role contributes every declared field or option when it holds the matching verb on the row's type and face: read for `visible:`, update for `fields:`/`options:`; new `acl.Request.RoleGrantsVerbOnFace` answers the verb under the client ceiling; history keeps its globals-only role set and treats "no block + read" as an affirmative grant of every field)
- [x] Regression test planned (table per dimension in roleunion_test.go, MEAS-ROLE-UNION-MONOTONIC property test over random role subsets, viewer-stays-restricted, history, ceiling still removes)
- [x] Related areas checked for similar issues (policy-wide checks ConditionallyVisible / FieldRestricted / QueryScopeVisibilityConflicts / aclaudit stay sound because a principal holding only the restricting role is still restricted; RelationVerdicts and RelationFieldVerdicts share the shape and get a follow-up bug; the BUG-WJSYQT branch's FieldVisibleOnEveryRow mirrors the old fold)
