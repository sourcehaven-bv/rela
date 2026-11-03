---
id: BUG-OFW8S0
type: bug
title: Readers could save settings defaults and trigger a git sync
description: PUT /api/v1/_settings and POST /api/v1/_git/sync wrote project-wide state without a permission check, so any signed-in principal could change everyone's create-form defaults or commit and push all pending changes.
priority: high
effort: s
why1: 'handleAPISaveSettings and handleGitSync ran their write for any signed-in principal: neither handler called an authorization check.'
why2: No permission existed to check. Both write state shared by the whole project rather than one entity, and the ACL only had entity verbs plus a few named global permissions such as config:edit.
why3: Entity writes are authorized centrally in entitymanager, but these handlers write the settings store and the git working tree directly. They fall outside the central gate, so each would have needed its own check, and none was added.
why4: Nothing flagged an HTTP handler that writes without an authorization guard. Review relied on authors noticing. The gap surfaced only when the archplan unguarded_entry detector listed entry points that write without passing a guard.
why5: Authorization of non-entity writes is opt-in per handler, with no inventory of entry points and no CI check that every write entry point declares and passes a guard.
prevention: Data-entry entry points carry //archplan:auth markers and authorization checks carry //archplan:guard markers, so archplan's unguarded_entry detector lists every write that passes no guard. A planned archplan ci gate (AM-archplan-unguarded-entry-gate) will fail the build on a new one. Tests in settings_authz_test.go and handlers_git_test.go and the decl-header goldens pin the 403s.
started: "2026-10-21"
completed: "2026-10-21"
status: done
---

## Description

Two data-entry write endpoints act on project-wide state but checked no
permission. Any signed-in principal, including one who may only read, could call
them:

- `PUT /api/v1/_settings` saves the create-form defaults. One set of defaults
serves every principal, so a reader could change what everyone's new entities
start with.
- `POST /api/v1/_git/sync` commits every pending change in `entities/` and
`relations/`, merges the remote and pushes. A reader could commit and push other
people's work.

`GET /api/v1/_git/status` also listed the paths of conflicting files to every
principal. Those paths name entities, and whether an entity exists is secret.

## Fix

- Two built-in global permissions: `settings:edit` and `git:sync`
(`internal/acl/policy.go`).
- `access.Mode.PermitsProjectWrite` decides a project-wide write: no policy
allows it, read-only refuses it, a configured policy asks the request's holder,
and every other mode refuses (fail closed).
- The settings save and the git sync return 403 naming the permission when it
is missing.
- The review found the palette save, the logo upload and delete, and the
theme import on the same settings page equally ungated. They need
`settings:edit` too.
- The git status reports `can_sync` and lists conflict files only to a
principal who may sync.
- The golden fixture grants `settings:edit` to its admin role; the settings,
palette, logo and theme-import goldens for the other identities now expect 403.

## Found by

The archplan `unguarded_entry` detector, which lists HTTP entry points that
write without passing an authorization guard.
