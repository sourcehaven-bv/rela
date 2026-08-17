---
id: RR-P34E8J
type: review-response
title: 'RFC 9728 challenge (AC #9) is unsatisfiable as planned: JWT gate only emits WWW-Authenticate when header is literally Authorization'
finding: 'jwtgate.go:228 emits WWW-Authenticate: Bearer only when cfg.HeaderName == "Authorization", but the default is X-Auth-Assertion (cmd/rela-server/main.go:108). MCP clients send Authorization and RFC 9728 requires the challenge carry resource_metadata=. Adding a well-known endpoint alone does not satisfy AC #9.'
severity: significant
status: open
---

## Finding

AC #9 requires an unauthenticated request to `/api/v1/_mcp` to return 401 with
`WWW-Authenticate: Bearer … resource_metadata=…`. The plan assumed this follows
from adding a `/.well-known/oauth-protected-resource` endpoint. It does not.

`internal/dataentry/jwtgate.go:225-231`:

```go
// RFC 6750: advertise the scheme when the assertion rides the standard
// Authorization header ...
if strings.EqualFold(g.cfg.HeaderName, "Authorization") {
    w.Header().Set("WWW-Authenticate", "Bearer")
}
writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", ...)
```

Two problems:

1. **Conditional emission.** The default assertion header is `X-Auth-Assertion`
(`cmd/rela-server/main.go:108`), so in a default deployment the challenge is
never emitted at all. MCP clients authenticate with `Authorization: Bearer`, so
an operator would have to reconfigure the whole server's header just to make MCP
discovery work.
2. **No `resource_metadata` parameter.** Even when emitted, the bare `Bearer`
challenge lacks the RFC 9728 parameter that lets a spec-compliant MCP client
discover the authorization server.

## Resolution required

Extend the 401 path so the MCP route emits a parameterised challenge — `Bearer
resource_metadata="https://<host>/.well-known/oauth-protected-resource"` —
independently of the configured assertion header name. Either extend
`JWTGateConfig` with an optional challenge override, or wrap the MCP handler
with its own 401 writer.

Add an explicit test asserting the parameter is present (AC #9), since a bare
`Bearer` would pass a naive "is the header set?" assertion.
