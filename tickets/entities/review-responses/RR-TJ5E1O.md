---
id: RR-TJ5E1O
type: review-response
title: Value rendering and template parsing differ from automation.Interpolate
finding: fmt.Sprint renders large floats as 1e+06; formatting differs from automation.Interpolate; templates are re-parsed per request including inside the Tx.
severity: nit
reason: 'Floats now render without an exponent (pinned by TestTaskItemValueText). Aligning every case with automation.Interpolate and parsing templates once at load are deferred: the templates are a few tokens, parsing costs microseconds per request, and the two renderers serve different inputs (stored values here, automation context there).'
status: deferred
---
