---
id: RR-WJ42HC6
type: review-response
title: Prompt injection through a flow step writes as run_as
finding: An llm action in a flow reads text others wrote and writes with run_as's grants, which may be wider than theirs.
severity: significant
resolution: 'Documented in GUIDE-flows and GUIDE-data-entry: grant run_as only the outputs'' writes and prefer schema-bounded outputs.'
status: addressed
---
