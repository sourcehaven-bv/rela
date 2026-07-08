---
id: RR-1GP8NI
type: review-response
title: 'Scope boundary: CLI analyze/validate use validator.Violation/CheckAll and will NOT show detail'
finding: internal/cli/analyze.go and validate.go and internal/analysis/analysis.go consume the separate validator.Violation struct (via CheckAll), not RuleResult.Violations. Under this plan that struct does NOT gain Detail, so CLI analyze/validate won't surface missing-header detail. Acceptable since the ticket targets the data-entry Analysis view only, but state it explicitly as a scope boundary so it isn't mistaken for an oversight. Also confirm no consumer JSON-marshals validation.Violation directly (grep showed field-name reads only -> safe).
severity: minor
resolution: 'Documented as explicit scope boundary (OUT): CLI analyze/validate use the separate validator.Violation/CheckAll path which does not gain Detail; data-entry Analysis view only. Grep confirmed no direct JSON-marshal of validation.Violation.'
status: addressed
---

See finding property.
