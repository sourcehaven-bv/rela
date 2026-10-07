---
id: RR-D93GA2
type: review-response
title: Duplicate DOM ids when an entity is in several sections
finding: Each row set id to the entity id, so the anchor was not unique.
severity: nit
resolution: anchorSections gives the id to the first row on the page only. rowAnchors.test.ts.
status: addressed
---
