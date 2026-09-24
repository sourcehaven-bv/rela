---
id: RR-W2UM25
type: review-response
title: Data-entry validator gate frozen at build time
finding: app.go builds the validator once; choosing DeclarativeGate-or-Ungated at wiring can freeze to Ungated and make the violation list an oracle.
severity: significant
resolution: The data-entry validator uses a late-bound gate (traversalGateFromContext per call; refusing fallback). MCP GatedReads gets reader and gate from one helper with three tiers (Unrestricted+Ungated; Policy+DeclarativeGate; Deny+refusing gate). Never Ungated under a principal.
status: addressed
---
