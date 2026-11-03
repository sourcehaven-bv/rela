---
id: BUG-T6HNKS
type: bug
title: Conflict routes are not ACL-gated (content disclosure)
description: Conflict routes skipped every permission check and showed a hidden entity's path and values. They now need git:sync.
priority: high
effort: s
why1: The conflict list and detail handlers called no permission check. They returned every conflicted file's path, entity id and both sides of its values to any signed-in principal.
why2: The store cannot parse a file that still has conflict markers, so these routes read the files directly. That bypasses the visibility readers that gate every store-backed read.
why3: The routes predate git:sync and the visibility wrappers. When git:sync was added for POST /_git/sync, and later for the conflict paths in GET /_git/status, the conflict routes were left out.
why4: A route that reads project files outside the store has no default gate. Nothing marks it as needing one.
why5: Read-side ACL is enforced by wrapping the store. Code paths that do not go through the store are protected only if someone remembers to add a check.
prevention: The conflict routes now share the git:sync check with the sync. TestV1Conflicts_GitSyncGate and the golden cases conflicts.* and conflict-hidden.* pin every route and ACL mode. archplan auth markers record the gate on each handler.
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

## Description

The data-entry conflict routes skipped every permission check. In the TKT-0M4H0S
golden corpus, the viewer got the diff of hidden TKT-4, including the hidden
`secret` value and its content (`conflict-hidden.viewer`). `GET /_conflicts`
listed the path and id of TKT-4 to every principal. BUG-Q3Z15V reports the same
defect for the list and the detail.

## Fix

The list (`GET /_conflicts`), the detail (`GET /_conflicts/{path}`) and resolve
(`POST /_conflicts/resolve`) now need `git:sync`, the permission `POST
/_git/sync` already needs. They share `permitsGitSync`
(`access.Mode.PermitsProjectWrite`). Without it they answer 403 with problem
type `permission_required` and name the permission. Resolve keeps its write
check on top.

The SPA hides the "Resolve conflicts" link when `can_sync` is false, and
`ConflictsView` shows a message naming `git:sync` on a 403.

## Why not row-gating

Row-gating each file through visibility was considered and not chosen. Merge
conflicts arise only on file (fs) storage, which runs locally. Network deploys
run on postgres or sqlite, which have no conflicts. So the coarse `git:sync`
gate is accepted: a holder of `git:sync` still sees the paths and unredacted
values of conflicting entities they may not read. The acl-security guide says
so.
