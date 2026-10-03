---
id: RR-WABHTW
type: review-response
title: Version origin label can name an unreadable face
finding: gateOriginSources labeled Source@SourceFace when some face was readable, telling a published-only reader that a draft exists (security review).
severity: minor
resolution: A label naming a face now needs verdict.Contains(face). Pinned by TestGateOriginSources_FaceLabelNeedsThatFace.
status: addressed
---
