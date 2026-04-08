---
id: RR-CM2P
type: review-response
title: Backend applyV1Filters fails open on malformed/unknown operator
finding: 'Two issues in internal/dataentry/api_v1.go applyV1Filters: (1) A key like filter[prop][][weird]=x parses to operator='''', falls through to the switch''s default case which appends every entity — silently disabling the filter. (2) Unknown operators (typo like filter[status][equals]=done instead of [eq]) hit the same default case. Fail-open filtering is dangerous: a configured scope filter could be bypassed by a typo or crafted URL. Fix: validate parsed shape (reject empty property/operator), and change the default switch case to log a warning and SKIP the filter entirely (don''t include all entities).'
severity: critical
resolution: 'api_v1.go applyV1Filters now: (1) validates parsed shape — rejects >2 segments, empty property, empty operator — and skips the filter with slog.Warn; (2) validates the operator against a known allowlist (eq/ne/contains/in/lt/lte/gt/gte) BEFORE the per-entity loop and skips unknown operators; (3) the inner switch''s ''default'' fail-open branch is removed as it''s now unreachable. Updated TestV1FilterUnknownOperator to assert fail-closed semantics, added TestV1FilterMalformedKeySkipped for empty property and extra-segment cases. All filter tests pass.'
status: addressed
---
