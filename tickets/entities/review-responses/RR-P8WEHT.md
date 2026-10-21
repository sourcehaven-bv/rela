---
id: RR-P8WEHT
type: review-response
title: Deprecated JWTPrincipalResolver does not bound streams by token expiry
finding: 'internal/dataentry/router.go:627: JWTPrincipalResolver verifies the same assertion but returns only a Principal, so it never puts the expiry on the context and an SSE stream opened under it stays open past the token, as before this change. cmd/rela-server does not wire it (main.go:364 uses SetJWTGate or the header/env chain), so rela itself is not affected; only a third-party embedder is. Fix: add one sentence to its Deprecated note saying event streams opened through it are not ended at token expiry, so an embedder does not assume the TKT-5I6REX guarantee.'
severity: nit
resolution: The Deprecated note on JWTPrincipalResolver says streams opened through it do not end at token expiry, because only the gate records it.
status: addressed
---
