---
id: RR-UQ73Q
type: review-response
title: Unreachable nil-check in luaURL
finding: 'internal/lua/urls.go:24-28 guards r.routes == nil, but runtime.go:585 only registers the binding when r.routes != nil. Dead code. Also urls_test.go:64 accepts two possible gopher-lua error strings (brittle across versions). Fix: delete the r.routes nil guard in luaURL.'
severity: nit
resolution: Dropped the r.routes == nil guard in luaURL; replaced with a comment pointing to runtime.go where conditional registration lives. The urls_test.go:TestURL_notRegisteredWithoutOption check still validates the absent-binding path via the Lua VM's own error string.
status: addressed
---
