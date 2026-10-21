---
id: BUGA-J6R7BG
type: bug-analysis-checklist
title: 'Analysis: Readers could save settings defaults and trigger a git sync'
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Settings: on develop the decl-header goldens `settings-put.viewer.golden`,
`settings-put.outsider.golden` and `settings-put.stranger.golden` record
`status: 200` for a `PUT /api/v1/_settings` by a viewer, an outsider and a
stranger under the golden fixture's `acl.yaml`.

Git sync: a throwaway test on a detached develop checkout configured a policy
ACL, attached a read gate that holds no permission, and called `handleGitSync`
on a repository with one uncommitted entity file. Result: `status=200
commits_added=1`. A principal without any permission committed the working tree.

Conditions: a configured `acl.yaml` (policy mode). Without `acl.yaml` every
principal may write every entity, so the open endpoints added nothing there.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: two built-in global permissions, `settings:edit` and `git:sync`,
decided by one `access.Mode.PermitsProjectWrite` (no policy allows, read-only
refuses, policy asks the holder, every other mode refuses).

Regression tests: `TestSaveSettings_Gate`, `TestHandleGitSync_Gate` and
`TestHandleGitStatus_CanSyncAndConflictFiles` in `internal/dataentry`, the
`PermitsProjectWrite` rows in `TestMode_Decisions`, and the settings goldens.

Related areas: the security review found the palette save, the logo upload and
delete, and the theme import on the same settings page writing shared state with
no gate, including under `--read-only`. They are gated by `settings:edit` in
this fix. `GET /api/v1/_conflicts` and `/api/v1/_conflicts/{path}` serve
conflict paths and content to every principal; that is the existing BUG-T6HNKS
and stays out of this fix.
