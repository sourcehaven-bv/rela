---
id: RR-MGV1QT
type: review-response
title: Panels and previews open a child without redirect
finding: EntityDetailPanel, EntityPreviewModal and SidePanel render an entity without going through the EntityDetail route, so the redirect does not apply and the child shows as a normal record there. The plan should state whether that is accepted.
severity: minor
resolution: 'Plan: accepted. Panels and previews show the child as a normal record with an ''in <parent>'' link and do not redirect.'
status: addressed
---
