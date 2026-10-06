---
id: RR-YHKWB8
type: review-response
title: MCP gated reader read full bodies for the subject scan
finding: gatedGraphReader had no ListEntityHeaders, so store.ListEntityHeaders fell back to ListEntities and loaded every subject body.
severity: significant
resolution: gatedGraphReader.ListEntityHeaders forwards to the gated row reader header path. TestScriptReader_CardinalityReadBudget pins equal gated reads at 10 and 50 subjects.
status: addressed
---
