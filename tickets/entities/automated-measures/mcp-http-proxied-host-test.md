---
id: mcp-http-proxied-host-test
type: automated-measure
title: 'MCP HTTP handler serves a proxied public Host on a loopback listener'
kind: test
location: internal/mcp/http_test.go (TestHTTPHandlerAcceptsProxiedHost)
status: active
description: >-
  Serves Server.HTTPHandler() on a loopback httptest listener and sends an
  initialize request with a non-loopback Host, the production topology behind
  pratique. Fails with 403 if the go-sdk DNS-rebinding guard is re-enabled.
---
