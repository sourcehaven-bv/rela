---
id: RR-QB2M51
type: review-response
title: InboxView.MarshalJSON lists its fields a second time by hand
finding: 'internal/realtime/inbox.go:134. The unread-only branch re-declares Notice, Degraded, UnreadChannels and UnreadCount in an anonymous struct. If a field is added to InboxView later, it is silently missing from every kinds=["unread"] MCP answer, and nothing fails. v1.Inbox solves the same problem with pointer and omitzero fields. Fix: give the waiting fields omitzero/pointer semantics on the realtime type (or render MCP through a small wire struct, as REST does), or add a test that marshals an unread-only view with every exported field set and asserts the key set.'
severity: nit
resolution: MarshalJSON marshals the struct and drops only listed waiting keys; TestInboxView_MarshalJSON checks every field by reflection
status: addressed
---
