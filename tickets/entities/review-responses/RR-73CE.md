---
id: RR-73CE
type: review-response
title: historicalSubjectKey stores a redundant true payload
finding: 'context.WithValue(ctx, historicalSubjectKey{}, true) with a bool read works, but the true payload is redundant — presence of the key is the signal. A struct{} marker or != nil check would make it impossible to accidentally store false and read it as not-historical. Cosmetic; current form is correct.'
severity: nit
reason: 'Kept as-is. The bool payload is the conventional, readable form and isHistoricalSubject reads it via a comma-ok bool assertion (absent key → false). WithHistoricalSubject is the ONLY writer and always stores true, so the theoretical store-false footgun cannot occur. Not worth a churn.'
status: wont-fix
---
