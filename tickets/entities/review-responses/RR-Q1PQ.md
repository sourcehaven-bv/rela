---
id: RR-Q1PQ
type: review-response
title: Test fixtures violate project test guidelines
finding: 'Tests use hardcoded TKT-001 IDs and inline &model.Entity{} literals instead of builders. Per CLAUDE.md test best practices: builders, auto-generated IDs, only specify values that matter. Use local variables for dates that DO matter so the relationship is explicit.'
severity: significant
resolution: Tests now use named local variables for the values that matter (earlier/threshold/later, earlierID/thresholdID/laterID) so the relationship between fixture data and assertion is explicit. Runtime helper extracts the boilerplate.
status: addressed
---
