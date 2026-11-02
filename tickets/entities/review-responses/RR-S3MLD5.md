---
id: RR-S3MLD5
type: review-response
title: MCP test can pass vacuously on the no-session branch
finding: internal/mcp/tools_channels_test.go:927-934 (TestReadChannel_In) loops over readMessageTypes(..., "room") and only asserts inside the loop. If message_types is not served (for example a change to the schemas default for cursorless reads), the map is empty and the "neither key" assertion silently checks nothing. Assert that the room map is non-empty before the loop, as the REST test does with require.Equal(len(wants), seen).
severity: minor
resolution: "addressed\tTestReadChannel_In now fails when read_channel serves no message_types for room, before the loop asserts that neither key is present."
status: addressed
---
