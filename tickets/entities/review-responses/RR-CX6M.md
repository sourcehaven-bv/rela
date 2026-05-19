---
id: RR-CX6M
type: review-response
title: Refuse startup on bad combinations (non-loopback + header)
finding: Cranky suggested converting documented risk into configuration error — refuse to start when --bind is non-loopback AND --principal-header is set, unless an explicit --allow-untrusted-principal-header is also given.
severity: significant
reason: I added a loud slog.Warn for the same combination, which addresses the in-band "tell the operator" concern. Promoting it to a hard refusal would break legitimate deployments behind well-configured reverse proxies — the proxy terminates non-loopback traffic, and rela-server itself is bound loopback from the proxy's perspective only if it's co-located. The leverage suggestion is correct for the direct-exposure case but would over-fire for the proxied case. Revisit if operator confusion shows up in practice.
status: deferred
---
