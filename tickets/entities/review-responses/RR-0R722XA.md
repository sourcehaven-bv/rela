---
id: RR-0R722XA
type: review-response
title: Relay origin check relies on outer middleware
finding: AllowedOrigins is '*' because requireSameOrigin wraps the outer mux. Nothing in handleCollab enforces that the wrapper is present.
severity: nit
resolution: _config test now goes through NewRouter with Host and Origin checks, the relay route is only reachable through the same middleware.
status: addressed
---
