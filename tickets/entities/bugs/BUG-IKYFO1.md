---
id: BUG-IKYFO1
type: bug
title: 'Remote MCP returns 403 behind proxy: go-sdk localhost guard rejects public Host'
description: 'Every remote MCP request on atlas returned `403 Forbidden: invalid Host header "atlas.sourcehaven.internal"`, after pratique had accepted the bearer token. pratique connects to rela-server over 127.0.0.1 and forwards the public Host. The go-sdk StreamableHTTPHandler enables DNS-rebinding protection by default, and it rejects a non-loopback Host on a loopback listener. Fixed by setting `DisableLocalhostProtection: true` in `Server.HTTPHandler()`.'
priority: high
effort: xs
why1: A Claude Code client with a valid pratique token got 403 `invalid Host header` from POST /api/v1/_mcp on atlas.
why2: 'The go-sdk StreamableHTTPHandler rejects a request when the connection''s local address is loopback and the Host is not. On atlas, rela-server binds 0.0.0.0, pratique connects over 127.0.0.1 and forwards the public Host atlas.sourcehaven.internal.'
why3: '`Server.HTTPHandler()` used the SDK defaults (`Stateless: true` only), and the guard is on by default since the v1.7.0 migration (TKT-UIR41P).'
why4: The remote transport was tested only through the in-memory transport and handler-level calls, never over a real loopback listener with a proxied Host, so the production topology was not exercised before deploy.
why5: The SDK guard assumes an unauthenticated local server. rela's endpoint is the opposite case (JWT-authenticated, behind a reverse proxy), and nobody checked the SDK's default security middleware against that deployment shape during the migration.
prevention: '`TestHTTPHandlerAcceptsProxiedHost` serves `HTTPHandler()` on a loopback httptest listener and sends `initialize` with a public Host. It fails with 403 without the fix. The comment on `HTTPHandler()` records why the guard is off, so it is not re-enabled by a later SDK options refactor.'
status: done
---

## Fix

`internal/mcp/server.go`: `StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true}`.

The guard defends unauthenticated local servers against DNS rebinding. The
remote endpoint only mounts with verified JWT identity (`-mcp` refuses to start
otherwise), and a rebinding page cannot present a bearer token.
