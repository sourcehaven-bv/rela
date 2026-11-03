---
id: RR-6FXAPOL
type: review-response
title: A DTSTART with a TZID east of UTC lands on the previous UTC date
finding: With DTSTART;TZID=Europe/Amsterdam at local midnight the occurrence is 22:00Z the day before.
severity: significant
reason: Pre-existing in metamodel.NextRrule, which already served predicates before this change, and unrelated to the missing-DTSTART defect. No Atlas rule uses TZID today. A separate ticket can decide between truncating in the DTSTART location or rejecting non-UTC DTSTART.
status: deferred
---
