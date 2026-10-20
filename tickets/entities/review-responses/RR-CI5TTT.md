---
id: RR-CI5TTT
type: review-response
title: Own-message buttons cannot be derived from the head flags
finding: '§0 "edit and delete from edit_own / delete_own plus authorship", AC 7. `affordancesWith` (internal/realtime/service.go:202) sets EditOwn from the gate and open state only. The server still refuses with an edit window (`EditOwn.Within`, TypeEditWindowClosed), when the caller can no longer post that type (`mayPost` in Edit), for a pending action (`requireNotPending`), and, for delete, with `delete_own.from`. `sameAuthor` also compares OrgID, which the wire author lacks. The SPA would show buttons that always fail. Fix: have the server add per-message affordances, e.g. `edit`/`delete`/`remove` entries in `Message._actions` or separate booleans, computed by the same checks as the operation. Do this in T7a, since §0 already treats affordances as the source of truth. Pin it with an AC: an edit button disappears after `within`, and on a pending message.'
severity: significant
resolution: T7a adds edit, delete, remove and release to each message's _actions, computed by the same checks as the operations, and the SPA shows message buttons only from them (R6, AC 10).
status: addressed
---
