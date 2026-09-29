---
id: AM-mcp-remote-reads-resolve-worlds
type: automated-measure
title: 'Test: remote MCP reads resolve faced entities through the world'
description: 'Drives the remote MCP read deps through the real router and ACL gate: a bare id resolves through the default world, ID@face selects that face, and a face-restricted reader gets only the face its grant allows. Catches a read surface that drops back to the default world (BUG-6XTX0G).'
kind: test
location: internal/dataentry (TestRemoteMCPDeps_FaceRestrictedReaderGetsTheFaceTheyMayRead, TestMCPReadWorld_ThroughTheRouter); internal/worldreader bound_test.go
status: active
---
