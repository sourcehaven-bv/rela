---
id: RR-SYNCR3
type: review-response
title: 'Pull must mirror a delete ONLY on a manifest Deleted:true tombstone, never infer deletion from a bare GET 404'
finding: 'The row-gate is unified in spirit (manifest filterVisibleManifest and handleV1GetEntity both resolve through readGateFromContext), but the two gates evaluate at different times against a mutable graph. Between manifest-read and fetch, an entity can become hidden (ACL-governing field changes, source re-typed/deleted for a relation). A became-hidden-after-feed row yields feed-says-changed -> GET-404. The current pull path treats a fetch 404 as errRemoteAbsent and MIRRORS it as a local delete (pull.go applyRemote). That turns a redaction flip into a spurious local deletion — the same delete-vs-hidden confusion as the field case, at row granularity.'
severity: significant
status: addressed
resolution: 'applyOne (internal/cli/sync/pull.go) mirrors a local delete ONLY on an explicit manifest tombstone (ch.Deleted). For a NON-tombstone entry whose fetch returns errRemoteAbsent (bare 404), it now returns OutcomePullSkipped ("became hidden or transient; left local copy intact") instead of aborting or deleting. ForcePull keeps mirroring a 404 as a delete, which is correct there (explicit operator make-local-match-remote).'
---

## Finding (design-review, fancy-browser)

The replica must distinguish 404-because-deleted from 404-because-now-hidden. It
should mirror a delete **only** when the manifest entry itself is `Deleted:true`
(the explicit tombstone), and must **never** infer a deletion from a bare GET
404. A GET 404 on a row the feed reported as changed (not deleted) means
"became hidden / transient" -> skip+advance, leave the local copy intact.

## Recommended resolution

Gate local-delete mirroring on `manifestChange.Deleted == true`. On a bare fetch
404 for a non-tombstone feed entry, skip+advance without deleting locally. Test:
a row visible at manifest time, hidden before the GET, does NOT get deleted from
the replica.
