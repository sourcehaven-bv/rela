---
id: RR-1CV4Y7B
type: review-response
title: Content-scoped edges tested only on the first hop
finding: content_scoped_edges_are_not_followed_on_a_step seeds a draft-face edge only from the source; later and incoming hops are not pinned.
severity: minor
reason: Every hop filters from_face in the same builder code as hop one; extra conformance cases are follow-up hardening.
status: deferred
---
