---
id: RR-QD5JRS
type: review-response
title: Editor reference links carry no world
finding: milkdown/entityRefNode.ts builds /entity/<type>/<id> without a world (and without relaBase()); its titles now resolve in the request world, so an editor link can open a different face than its title. The mentions.go comment claimed the world for every SPA link.
severity: significant
resolution: Narrowed the mentions.go comment to the read view (rewriteEntityRefToken).
reason: 'Out of the agreed scope: the requester limited the world-on-link change to the read view in frontend/src/utils/markdown.ts. Edit forms are not world-bound (writes refuse ?world=), so the editor reads in the default world and its links already open there; the missing relaBase() predates this change and belongs in a separate ticket.'
status: deferred
---
