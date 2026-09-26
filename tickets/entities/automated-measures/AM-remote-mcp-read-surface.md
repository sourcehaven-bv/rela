---
id: AM-remote-mcp-read-surface
type: automated-measure
title: 'Integration test: remote MCP offers no Lua tools and search returns only readable rows'
description: Builds the remote MCP server through newRemoteMCPServer under an acl.yaml policy and asserts tools/list has no lua_* tool, a lua_* call fails, and search_entities drops unreadable rows, hidden-field-only matches and hidden titles. Catches a networked wiring that reuses stdio deps (the class that produced BUG-RIJR6R).
kind: test
location: cmd/rela-server/mcp_remote_test.go
status: active
---
