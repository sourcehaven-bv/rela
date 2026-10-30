---
id: RR-G4MDBG
type: review-response
title: Serve the entry index on the transition row instead of relying on row order
finding: 'AC 7 matches {message_type, entry} to the n-th row of its type in session.transitions. That includes by: trigger rows, which decisionViews drops before it groups (channelView.ts:401), so the SPA must count rows before it filters, a trap waiting to be stepped in. MCP agents have the same problem. Add entry to SessionTransitionView (internal/realtime/session.go:99-113) and to the wire type. Transition.index is unexported (internal/realtimecfg/session.go:243), so the plan also needs an Index() accessor, which it does not list.'
severity: minor
resolution: Each session.transitions row now carries entry, read through a new Transition.Index() accessor, and the SPA matches entries by that field instead of row order.
status: addressed
---
