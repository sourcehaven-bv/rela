---
id: RR-U4PH2Z
type: review-response
title: transitions must be [] on every return path
finding: 'affordancesWith returns early for a caller without identity with only Post and Lifecycle set (internal/realtime/service.go:212-215). wireHead nil-guards only Post and Lifecycle (internal/dataentry/channels_wire.go:127-132). Without both changes, transitions is null on the wire, which breaks the plan''s "never null" and the OpenAPI "required array". Plan change: name both sites in Approach 2, and add a no-identity row to AC 4.'
severity: nit
resolution: 'Plan updated: affordancesWith sets Transitions: []string{} before the no-identity return and wireHead nil-guards it (Approach 3); AC 4 adds a no-identity row expecting "transitions": [].'
status: addressed
---
