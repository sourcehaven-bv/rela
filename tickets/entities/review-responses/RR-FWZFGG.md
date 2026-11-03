---
id: RR-FWZFGG
type: review-response
title: Replay resolves the principal twice and targets the wrong mux
finding: The SSE ctx holds the already-resolved principal, so a replay through attachACLRequest would resolve again and could run as another user; the ACL wrappers sit on the outer mux, which also routes _events, hooks and the SPA.
severity: significant
resolution: A middleware saves the pre-resolution principal for replay; the replay target is a dedicated mux built by registerAPIV1Routes wrapped in the reload lock, attachWorld and attachACLRequest. New AC10 parity test (redacted field, world path, principal_property principal).
status: addressed
---
