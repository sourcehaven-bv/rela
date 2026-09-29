---
id: RR-YXB2C4
type: review-response
title: No tests for the relation-history face gate
finding: authorizeRelationHistoryRead and serveRelationHistoryVersion gained face checks but no test covered a denied tail face or the no-meta path.
severity: significant
resolution: perEndpointGate now carries readable faces per type. TestRelationHistory_DeniedTailFaceIsNotFound covers the list and version routes. TestRelationHistory_GoneSourceServesNoMeta covers the no-meta path.
status: addressed
---
