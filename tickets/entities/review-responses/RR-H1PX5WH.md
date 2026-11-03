---
id: RR-H1PX5WH
type: review-response
title: List the differences between GetAddress and ResolveHeadersErr
finding: 'Design review: a family stored under two types, a header-read error versus a miss, and DenyReader behaviour may differ between the two reads.'
severity: minor
resolution: A bare ref resolves in the reader's world, as GetAddress did; TestAnalyze_FacedValidationViolators pins the faced case for two principals. A header-read error now becomes an error issue instead of a silent skip. DenyReader serves nothing, as before. The atlas response is byte-identical to develop for two principals.
status: addressed
---
