---
id: RR-FMTJRQ5
type: review-response
title: Write-op allowlist not pinned
finding: The plan did not list the ops, keyed on op alone, and left new ops unclassified.
severity: significant
resolution: 'Plan: explicit allowlist in the plan, op filter pushed into SQL, Kind must be entity or relation, and a test that fails for any audit.Op constant not classified.'
status: addressed
---

The plan did not list the ops, keyed on op alone, and left new ops unclassified.
