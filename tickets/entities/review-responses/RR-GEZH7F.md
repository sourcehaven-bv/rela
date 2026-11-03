---
id: RR-GEZH7F
type: review-response
title: 'MCP path has no unkeyed-type case under default_in: keyed'
finding: 'internal/mcp/tools_channels_test.go:893-898: defaultInYAML gives text a written in:, so under default_in: keyed every breach type is derived or written, and the MCP test never checks a type that default_in leaves without in (the "no state keys it" row of the wire table). TestDescribe_In and TestChannelSchema_In (reminder) cover it on the shared struct, so the risk is low; adding a fifth breach type with no on: key, or asserting that a room type stays bare after the swap, would close the AC 5 row.'
severity: nit
resolution: "addressed\tdefaultInYAML adds a note type that no state keys; TestReadChannel_In asserts it has neither in nor in_derived under default_in: keyed."
status: addressed
---
