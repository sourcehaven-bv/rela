---
id: lua-sandbox-tests
type: automated-measure
title: Lua sandbox security tests
description: Security tests verifying Lua scripts cannot escape the sandbox
kind: test
location: internal/lua/runtime_test.go
status: active
---

# Lua Sandbox Security Tests

Security tests verifying Lua scripts cannot:
- Access filesystem outside project
- Execute system commands
- Access network
- Escape the sandbox
