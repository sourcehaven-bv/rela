---
id: RR-H2WQA7
type: review-response
title: transition_removed result always has kind set
finding: 'internal/realtime/outputs.go:455-458: when the transition is removed, the appended result is Kind set even when the pending outputs were creates, and the CLI then renders "set <anchor> (transition_removed)". Fix: use the kind of the first output without a result if it is known, or add a distinct kind or no kind for this case and document it in OpenAPI/TS.'
severity: minor
resolution: transition_removed has the new kind transition across channels, realtime, apiwire, OpenAPI, TS and docs.
status: addressed
---
