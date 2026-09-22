---
id: RR-T7Z6UW
type: review-response
title: Download streams via io.Copy with no size gate or Range support
finding: 'Code review: handleCommandFile streams with io.Copy and no size limit, while the attachment and export paths have size gates. dataentry/CLAUDE.md records that the custom/ handler switched to http.ServeContent for exactly this — it gives conditional requests, Range, and correct HEAD in one call, though it requires keeping the explicit Content-Type (ServeContent sniffs otherwise, defeating nosniff) and adding a pre-read size gate.'
severity: minor
reason: Real and worth doing, but out of scope for a ticket whose stated scope is replacing the launcher. The bytes come from an operator-authored script writing inside the project root, so the producer is already trusted and already able to fill the disk directly — this route does not widen that. Bundling a ServeContent migration plus a size-gate policy into a security-sensitive ACL change would mix two independent risks in one diff. Deferred to a follow-up that can also reconcile the size limits across all three download endpoints, which is the actual inconsistency.
status: deferred
---
