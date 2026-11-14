---
id: RR-U5V89VY
type: review-response
title: schema.<type> prefix belongs in the CLI
finding: Custom type names are not validated, so the prefix should not leak into HeadlessFlow.Name or reach the run store.
severity: minor
resolution: HeadlessFlow.Name stays the custom type name. The schema. prefix is one exported constant, used by the CLI only.
status: addressed
---
