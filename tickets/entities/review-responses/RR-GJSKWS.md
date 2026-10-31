---
id: RR-GJSKWS
type: review-response
title: read_session description points to message types it does not serve
finding: tools_channels.go:340 reuses presetNote ('add the message type's other required properties') in read_session, whose own description says it serves no message types. Add a pointer to read_channel for the type's required properties.
severity: nit
resolution: "addressed\tThe read_session description now adds that read_channel lists the message type's properties. The MCP golden file is updated."
status: addressed
---
