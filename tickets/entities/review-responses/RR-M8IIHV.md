---
id: RR-M8IIHV
type: review-response
title: Incoming card path is dead on this branch with no guard — renders blank until ODHV2D merges
finding: 'The list endpoint Kanban calls serializes only OUTGOING edges into a row''s `relations` map (api_v1.go:321 → toV1 entityserializer.go:52-57, keyed by edge.Type). Incoming edges are keyed under an inverse name only in the grouped relations-detail handler, which Kanban never calls. So an incoming card field computes key `X_inverse`, finds it absent from relations, and silently renders blank. `included` carries the source (ACL-filtered) but the card never reads it because the incoming KEY is missing. This is the documented ODHV2D dependency — but there is NO guard: half the acceptance criteria render blank on any develop-based build before ODHV2D merges, with no feature flag, affordance, or validation error. Fix: land NC3D08 together with ODHV2D, OR gate/affordance the incoming path so an operator gets an explanation, not a blank field. KanbanView.vue:273-288.'
severity: critical
resolution: 'Made the ODHV2D merge-order dependency explicit and honest (no fake flag): a MERGE-ORDER DEPENDENCY comment at KanbanView.vue''s relationCardKey resolution site naming the inverse-key contract, and a Dependencies note in the ticket body. Incoming fields degrade VISIBLY to the existing ''-'' placeholder (getCardFieldValue returns '''') rather than a silent blank. The real enforcement is the contract test in RR-UKS8BW. Commit 5a0f8e0a.'
status: addressed
---
