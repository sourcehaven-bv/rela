---
id: TKT-Z1OP7R
type: ticket
title: Define a script security/capability model (http on read path, SSRF, local-service egress)
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Summary

Review finding **C5**, deferred deliberately: rela's Lua scripts have an
**implicit, scattered** security posture rather than an explicit capability /
sandbox model. The concrete symptoms below are real, but patching them piecemeal
would add more scattered decisions. They should be resolved together as one
articulated model — what a script may reach, on which path, and who decides.

## Concrete symptoms (the C5 finding)

1. **`http.*` is registered on reader runtimes.** `registerBindings`
(`internal/lua/runtime.go:636`) calls `r.registerHTTPModule()`
**unconditionally** — outside the `if allowWrites` guard that gates the
write/`write_file` bindings. So `NewReader` runtimes get outbound HTTP. The
**validation path** uses a reader runtime (`validator.New` → `lua.ReadDeps`), so
a validation rule evaluated against N entities can fire N outbound HTTP requests
— the same "per-entity cost with no quota" hazard the project already bans for
AI in validation (see `internal/ai/` docs) — and can be used as an SSRF pivot.

2. **No SSRF guard in `validateURL`** (`internal/lua/http.go`). It permits any
`http`/`https` host, including loopback (`127.0.0.1`), link-local /
cloud-metadata (`169.254.169.254`), and RFC-1918 private ranges. (It does
already reject non-http(s) schemes and userinfo.)

## Why deferred (not patched now)

The interesting question isn't "block these" — it's a **policy decision** that
belongs in a script security model that doesn't yet exist as an explicit thing:

- Should read-path / validation Lua be able to make HTTP calls at all, or is
network egress a write-path-only capability?
- Is a call to a **local service** (e.g. `127.0.0.1:NNNN`) a deliberate
integration the operator is fine with, or an SSRF to block? That's an
operator/deployment policy, not a hardcoded yes/no.
- Who configures the policy (per-deployment? per-runtime? acl.yaml-adjacent?),
and how does it compose with the existing implicit posture (`SkipOpenLibs` — no
io/os/debug; write-bindings gated to writers; AI self-guarding on readers;
`validateURL` userinfo rejection)?

Hard-gating http to writers + bolting an SSRF blocklist onto `validateURL` would
be two more scattered, implicit decisions — the exact pattern that produced the
inconsistency the review flagged. When this is scheduled, run `/research` (as
was done for the matcher convergence, RES-6PK0S3) to define the model first,
then implement.

## Scope when picked up

- Articulate the capability model: filesystem, network (http), AI — per read
vs write path — with explicit trust boundaries.
- Decide and implement the read-path http policy (gate vs opt-in vs
operator-configured).
- Decide and implement the SSRF posture (default-block private/loopback/
metadata with an opt-out for trusted local-service egress; dial-time IP check on
the shared transport to close the DNS-rebinding TOCTOU vs a simpler pre-resolve
check).
- Document the model so future script-reachable capabilities pick their path by
it rather than re-deciding ad hoc.

## References

- `internal/lua/runtime.go:617-637` (registerBindings — the unconditional http registration)
- `internal/lua/http.go` (validateURL — no SSRF guard)
- `internal/validator/validator.go:82` (validation uses the reader runtime)
- `internal/ai/` docs (the "no AI in validation — per-entity cost, no quota" precedent this generalizes)
- Found in the 2026-06-09 backend review (`.ignored/backend-review-2026-06-09.md`, finding C5).
