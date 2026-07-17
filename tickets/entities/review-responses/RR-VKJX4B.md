---
id: RR-VKJX4B
type: review-response
title: Entity update-vanished mapped to 404 while relation maps to 412 (asymmetric CLI abort)
finding: persistApplyEntity mapped an update whose row vanished (probe-said-present-then-deleted race) to ErrEntityNotFound -> 404, while the relation equivalent mapped vanished -> 412. On the CLI a 404 aborts the whole run; 412 halts one record. Asymmetric and undocumented.
severity: minor
resolution: Mapped sync entity update-vanished to 412 (option a), symmetric with relations. Verified the v1 PATCH 404 existence-oracle is a separate handler (writeSyncApplyError is sync-only), so unaffected. Introduced ErrEntityVanishedOnUpdate that WRAPS ErrEntityNotFound (existing errors.Is callers still match); handler checks it -> 412 before the generic ErrEntityNotFound -> 404 (order pinned by test). New test TestSyncPut_UpdateVanishedReturns412. Commit c95947da.
status: addressed
---
