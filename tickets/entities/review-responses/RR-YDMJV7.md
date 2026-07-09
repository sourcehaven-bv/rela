---
id: RR-YDMJV7
type: review-response
title: History snapshot read bypasses field-level (visible:) redaction — leaks hidden properties
finding: 'In this codebase field-level READ redaction is applied by the dataentry serializer layer (stripHiddenProperties in affordances.go:846, via entityserializer.go:112), NOT by the store or the ACL gate — the store returns the FULL entity. A version snapshot stores full content + all properties. The plan''s history read endpoint, as written, returns the snapshot without re-running stripHiddenProperties, so every visible:-denied property leaks verbatim, violating the documented invariant (docs/server-security.md:281, docs/acl-security.md:496). Secondary channels leak too: title is rewritten to the ID when the display property is hidden (affordances.go:862), and attachment metadata is gated on visibility (affordances.go:924). Fix: the history endpoint MUST construct an *entity.Entity from the snapshot and route it through entitySerializer.forWire so stripHiddenProperties runs; add a contract test (mirror affordances_contract_test.go) that every field absent from a live GET is absent from the version response. Also decide visible:when: predicate basis — evaluate against LIVE entity state (deny-all-conditional if deleted), not snapshot state.'
severity: critical
resolution: 'Implemented in slice 7 (dataentry): history read endpoints reconstruct an *entity.Entity from each snapshot and route it through entitySerializer.forWire, so stripHiddenProperties runs and visible:-denied properties never reach the client — never a raw snapshot. internal/dataentry/history_handler.go serveHistoryVersion.'
status: addressed
---
