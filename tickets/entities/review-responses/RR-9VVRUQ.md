---
id: RR-9VVRUQ
type: review-response
title: Delete ACL suites never run the guard pre-pass
finding: About 60 fixtures pass NopAnchorGuard, which skips the pre-pass. The production path (cascade, owned, ErrHasRelations authorization in the pre-pass, block on an owned child, owned race, errFacesChanged, purge undo, partial tombstone) has no test.
severity: significant
resolution: The cascade_authz, owning, familydelete_acl, faceedge_delete_acl and lastface tables also run with a real fake guard; anchor_guard_race_test.go covers denied owned entity, owned-child block, owned race, face change and purge undo.
status: addressed
---
