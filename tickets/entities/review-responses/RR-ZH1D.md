---
id: RR-ZH1D
type: review-response
title: Visible verdict semantics under closed-world were unspecified
finding: 'The draft listed `visible:` as a YAML block sibling but never spelled out the resolver logic. Under closed-world per-type opt-in, a `visible: {ticket: []}` block would silently hide every undeclared ticket field — stripping properties from wire AND from _title. UIs would render blank cards. No AC or test scenario covered this.'
severity: critical
resolution: 'Added explicit semantics: visible: follows per-type opt-in like fields: (declared block for type T is closed-world for T''s visibility; absent block = fully visible). Hidden takes precedence over read-only when both apply (matches affordances.go:585). Added ACs and integration test for closed-world visibility.'
status: addressed
---
