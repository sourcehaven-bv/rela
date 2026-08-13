---
id: BUG-0Q8MCZ
type: bug
title: syncContext drops every verified assertion claim (org, roles) via a plain composite literal
description: syncContext rebuilds the Principal with a plain composite literal, which cannot carry the unexported verified-assertion fields (orgID, orgSlug, roles) or RawUser. Under a verified-JWT deployment using asserted_role_assignments, every grant sourced from a signed identity assertion evaporates on the sync path. Same failure mode resolvePrincipalEntity explicitly guards against at router.go:380-382.
priority: medium
status: backlog
---

## Symptom

`internal/dataentry/sync_handlers.go:20-23`:

```go
func syncContext(ctx context.Context) context.Context {
	p := principal.From(ctx)
	return principal.With(ctx, principal.Principal{User: p.User, Tool: principal.ToolSync})
}
```

This rebuilds the Principal with a **plain composite literal**, which cannot
carry the unexported verified-assertion fields. So a sync request silently loses
`orgID`, `orgSlug`, `roles` — and `RawUser`.

## Impact

Under a verified-JWT deployment using `asserted_role_assignments`, every grant
sourced from a signed identity assertion **evaporates on the sync path**.
`internal/acl/resolver.go:57-65` is the only consumer of `Principal.Roles()`;
with an empty role slice it contributes nothing, so a principal whose entire
authority came from asserted roles is reduced to whatever `Assignments` /
membership gives it — typically nothing.

`RawUser` loss additionally degrades audit attribution for principals resolved
via `principal_property`.

## Root cause

This is exactly the failure mode `resolvePrincipalEntity` documents and guards
against at `internal/dataentry/router.go:380-382` — that call site uses
`principal.Verified(...)` precisely because a composite literal would drop org
and roles. `syncContext` is the same re-stamp pattern without the guard.

The trust-boundary design (unexported fields + `Verified()` as sole constructor,
`internal/principal/principal.go:58-95`) makes forging a claim impossible, but
it also makes *dropping* one easy and silent: a composite literal compiles fine
and zeroes them.

## Fix sketch

Re-stamp from the existing principal rather than rebuilding it — clone and
override `Tool`, or add a `principal.WithTool(p, tool)` helper so the re-stamp
idiom is available without reaching for a literal. A `Verified(...)` call
mirroring `router.go:383` also works but restates every field.

Worth considering whether the general shape (re-stamp Tool, keep everything
else) deserves one blessed helper, since this is the second site to need it and
the third would likely repeat the bug.

## Notes

Found while planning **TKT-IAC8TX** (client attenuation). That ticket adds
`principal_type` / `scope` to Principal; both would be dropped here too, which
would silently disable client attenuation on the sync path. Fixing this is not a
prerequisite for that ticket, but leaving it unfixed adds a second silently-
attenuated surface.

No test currently pins claim preservation across `syncContext` — the fix should
add one.
