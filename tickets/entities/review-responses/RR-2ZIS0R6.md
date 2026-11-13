---
id: RR-2ZIS0R6
type: review-response
title: Page past the end panics in the conflict list
finding: 'Cranky 2: start := (page-1)*perPage overflows for a huge page number and slices past the end.'
severity: significant
resolution: The page loop checks page-1 against the page count before computing the start; tests cover page 3 and math.MaxInt64.
status: addressed
---
