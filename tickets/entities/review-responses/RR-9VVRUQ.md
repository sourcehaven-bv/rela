---
id: RR-9VVRUQ
type: review-response
title: Delete ACL suites never run the guard pre-pass
finding: About 60 fixtures pass NopAnchorGuard, which skips the pre-pass. The production path (cascade, owned, ErrHasRelations authorization in the pre-pass, block on an owned child, owned race, errFacesChanged, purge undo, partial tombstone) has no test.
severity: significant
status: open
---
