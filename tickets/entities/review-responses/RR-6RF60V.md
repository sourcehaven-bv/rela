---
id: RR-6RF60V
type: review-response
title: Relation filter operators silently ignored (fail-open) — filter[rel][ne]=X returns whole list
finding: 'applyRelationFilters parses the relation key with TrimPrefix/TrimSuffix, so `filter[belongs_to][ne]` yields `belongs_to][ne` which GetRelationDef rejects and the pass skips it. Meanwhile applyV1Filters sees property=`belongs_to`, isRelationKey true, and also skips it. Neither pass handles the operator and no warning fires. Verified: `filter[belongs_to][ne]=Apollo` returns all rows (expected: not-Apollo). Property filters fail CLOSED with slog.Warn on unknown operators; relation filters fail OPEN and silent — the worst option. Fix: either parse the relation key with the same Split("][") logic and honor eq/ne/contains/in, or if operators are out of scope, detect an operator segment on a relation key and slog.Warn + fail closed. api_v1.go:310, :1934, :1955.'
severity: critical
resolution: New parseRelationFilterKey uses the same Split("][") logic as applyV1Filters so operator segments are recognized. eq and ne are supported; any other operator fails CLOSED (returns entities[:0] + slog.Warn), matching the property-filter contract. Pinned by TestV1ListRelationFilterOperatorFailsClosed (ne complement works; contains/unknown drop all rows). Commit 72f10b99.
status: addressed
---
