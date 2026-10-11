---
id: RR-BJ9DGD
type: review-response
title: An incoming move authorizes every visible sibling, even when only the moved edge is written
finding: authorizeIncomingSiblings authorizes all visible edges before the plan is known, so a midpoint move that writes one edge still fails when another source is denied.
severity: minor
reason: Deliberate. The plan is computed inside the Tx and authorizing there would put ACL reads under the write lock. Authorizing up front keeps the move consistent with movable, which already answers false for such a list, so the UI never offers a move that could fail halfway.
status: wont-fix
---
