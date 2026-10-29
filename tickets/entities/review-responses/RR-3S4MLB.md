---
id: RR-3S4MLB
type: review-response
title: MCP descriptions tell agents the first entry whose poster rule holds wins, but session.transitions does not show the poster rule
finding: 'tools_channels.go:309 branchingNote, used in read_channel, read_session and list_inbox. SessionTransitionView (realtime/session.go:158-162) carries by but not roles, so an agent cannot tell which entry applies to it. The roles are config and not secret (CLAUDE.md: the configuration is not a secret), so exposing roles per row is allowed. Either add roles to the row, or change the note to say that actions.transitions/transitions says which types the caller fires and that the target depends on the caller''s roles and the guard.'
severity: minor
resolution: The MCP branchingNote now says a row shows by but not the roles of its entry, so the target depends on the caller's roles and the message, and that transitions lists the types the caller can move the session with at all.
status: addressed
---
