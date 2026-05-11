---
id: RR-R8X6
type: review-response
title: V1View*Info naming lies about scope
finding: V1ViewAddInfo/V1ViewLinkInfo/V1ViewAddTarget are now used exclusively by V1SidePanelSection. The 'View' prefix is a tenant of the previous architecture. User explicitly deferred renaming, but a follow-up TODO/ticket reference in the type's comment would prevent silent re-bleed (a future contributor might wire 'ViewAddInfo' into a new view-related response struct because the name suggested it was generic).
severity: significant
resolution: Created follow-up ticket TKT-6ETQ for the rename to V1SidePanel*. Added doc-comment TODO references on V1ViewAddInfo, V1ViewAddTarget, and V1ViewLinkInfo in internal/dataentry/api_v1.go pointing to TKT-6ETQ and warning future contributors not to reach for these types from new view-related responses.
status: addressed
---
