---
id: RR-P751MM
type: review-response
title: Rollup must count children whose rollup property was redacted, not drop them
finding: visibility.Redact REMOVES a hidden property from Properties and records its name in e.Redacted (policyreader.go:242-252). A child whose rollup property is visible:-restricted therefore arrives with no value, indistinguishable from genuinely unset. If the fold skips valueless children, the segment counts stop summing to the visible child count, and the difference is itself an inference channel revealing that a restricted property exists on that child. The plan named the hidden-child row case but not the hidden-VALUE case.
severity: significant
resolution: 'Moot: rollup: is out of scope for this ticket (user decision). With no fold over a child property there is no segment arithmetic to leak, so the redacted-value bucket question does not arise. The finding is still correct and is carried onto the deferred-rollup follow-up ticket, because it is the non-obvious part of ever adding a rollup: visibility.Redact removes the property entirely (policyreader.go:242-252), so redacted-value children must still be counted or the segments stop summing to the visible child count.'
reason: 'Moot rather than rejected: the user descoped the rollup from this ticket (''fine to leave rollup out of it for now - my main request was the nested view anyway''). With no fold over a child property, there is no segment arithmetic and therefore no inference channel to close. The finding is not dismissed as wrong -- it is verified and correct, and has been carried verbatim onto TKT-ZAD9PS (Per-parent rollup bar), where it is documented as one of the two non-obvious problems that ticket must solve. Nothing is lost by closing it here.'
status: wont-fix
---
