---
id: RR-TDTGIT
type: review-response
title: Test fixtures for AC 3 and AC 5 are not chosen
finding: 'AC 3 (REST) and AC 5 (MCP) name tests but no fixture. The realtime test can use pocRealtimeDefaultIn, but that helper is in package realtime, so dataentry and mcp cannot use it. breachRealtime in internal/dataentry/channels_schema_test.go covers a written in: and a channel type without a session, but has no derived type. The MCP channel tests have no default_in fixture. Name the YAML for each test: for example, breachRealtime with default_in: keyed added and the in: removed from one keyed type. This gives derived, written, unkeyed and no-session rows in one answer. Also say that the MCP call passes no cursor, or passes schemas: true, because message_types is served only then.'
severity: minor
resolution: "addressed\tNamed defaultInRealtime (breachRealtime plus default_in: keyed and a written in: on text) and defaultInYAML (breachYAML likewise, room as no-session row); the MCP call passes no cursor. Correction: breachRealtime had no written message-type in:, only a reminder in:."
status: addressed
---
