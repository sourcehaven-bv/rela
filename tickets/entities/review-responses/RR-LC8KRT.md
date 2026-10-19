---
id: RR-LC8KRT
type: review-response
title: HTTP messages route now pays for affordances it discards
finding: Read always computes c.affordances() (read.go:112), so every GET messages now evaluates mayPost for each type plus the edit/delete gates. Before the switch to Read it evaluated none. With Schemas true, messageTypes (read.go:196) calls mayPost per type a second time. Compute the post set once and skip affordances when the caller does not render them.
severity: minor
resolution: ReadRequest.Actions makes the affordances opt-in, and ReadView.Actions is a pointer with omitempty. The post gate set is computed once (postable) and shared by affordances and message types, and is skipped when neither is asked for. Only the MCP read sets Actions.
status: addressed
---
