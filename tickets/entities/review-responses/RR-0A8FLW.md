---
id: RR-0A8FLW
type: review-response
title: Field filter ran under a row-only policy
finding: gatedSearcher always used SearchVisibleFields, loading every candidate even when no field can be hidden.
severity: significant
resolution: With a NopRedactor the searcher uses SearchVisible, mirroring dataentry searchScopedHits.
status: addressed
---
