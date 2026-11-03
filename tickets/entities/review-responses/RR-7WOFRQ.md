---
id: RR-7WOFRQ
type: review-response
title: No test pins the fail-closed path for a broken acl.yaml on stdio channel tools
finding: The plan's PolicyChannelService guarantee is "error, never an open fallback". mcp_wiring_realtime_test.go only covers no acl.yaml and a valid one. Nothing asserts that an invalid acl.yaml (parse error, or an unknown type against the schema) leaves the channel tools unregistered, or what happens to them on reload.
severity: significant
resolution: 'Added TestMCPStdio_BrokenACLAtStartup: an unparseable acl.yaml stops startup, and an acl.yaml that does not apply makes read_channel and post_message answer "acl.yaml cannot be loaded: ..." while show_entity works. Added TestMCPStdio_BrokenACLOnReload (new schema served, channel tools refuse), TestMCPStdio_ACLWatch and TestUnavailableChannels (all six tools refuse).'
status: addressed
---
