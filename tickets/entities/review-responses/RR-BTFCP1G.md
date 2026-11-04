---
id: RR-BTFCP1G
type: review-response
title: Marking an entity opened loads the whole pile
finding: MarkOpened runs PileByKey (full item load) plus MarkRead per auto-read pile.
severity: minor
reason: The client posts only when the entity is unread on an auto-read pile so it is rare. A mark-by-key store call is a later optimisation.
status: deferred
---
