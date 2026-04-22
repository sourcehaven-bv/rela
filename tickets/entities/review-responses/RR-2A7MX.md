---
id: RR-2A7MX
type: review-response
title: RelationCardsPage.hasAnyUnsavedBadge has dead catch
finding: .first() never throws; isVisible() returns false. Delete the .catch(() => false).
severity: nit
resolution: Removed the .catch(() => false) from RelationCardsPage.hasAnyUnsavedBadge.
status: addressed
---
