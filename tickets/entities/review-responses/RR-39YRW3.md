---
id: RR-39YRW3
type: review-response
title: Empty -jwt-header silently denies every API request
finding: 'validateIdentityFlags did not reject an empty -jwt-header. With HeaderName empty, r.Header.Get("") returns "", so every assertion reads as absent and every /api/ request is denied 401. The failure is fail-closed but silent: the server boots cleanly, logs nothing unusual at startup, and then 401s everything — a total outage whose cause is invisible in the startup output. Reachable by passing -jwt-header="" explicitly (the flag otherwise defaults to X-Auth-Assertion).'
severity: significant
resolution: 'validateIdentityFlags now returns an error when f.jwtHeader is empty and JWT identity is enabled, converting a silent total outage into a startup refusal with a clear message. Belt-and-braces: SetJWTGate also rejects an empty HeaderName. Covered by a new case in TestValidateIdentityFlags and verified against the real binary.'
status: addressed
---

Reported by cranky-code-reviewer against `cmd/rela-server/main.go:169-186`.

Verified live after the fix:

```
level=ERROR msg="invalid identity configuration"
  error="-jwt-header must not be empty when jwt identity is enabled"
```
