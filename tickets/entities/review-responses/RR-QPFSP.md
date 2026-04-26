---
id: RR-QPFSP
type: review-response
title: 'F2: allowFullDetail intentionally ignores X-Forwarded-For'
finding: internal/dataentry/script_errors.go:60-69 reads only r.RemoteAddr. There is no X-Forwarded-For middleware in internal/dataentry/, so an attacker cannot spoof loopback. Behind a reverse proxy this fails closed (proxy IP is non-loopback → degraded shape), which is correct given the data-entry server has no auth. Worth a comment so a future maintainer adding proxy-aware middleware doesn't break the gate.
severity: nit
resolution: Added a comment to allowFullDetail stating it intentionally ignores X-Forwarded-For, fails closed behind a reverse proxy, and that any future proxy-aware middleware must keep this gate honest.
status: addressed
---
