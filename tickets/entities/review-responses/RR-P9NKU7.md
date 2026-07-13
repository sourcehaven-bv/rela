---
id: RR-P9NKU7
type: review-response
title: Display vs edit disagree on naive (offset-less) stored datetimes
finding: 'formatDatetime parses with native new Date(value) (interprets a naive ''YYYY-MM-DDTHH:MM:SS'' string in the browser/runtime zone), while utcISOToLocalInput parses with new TZDate(iso, tz) (interprets it in the effective display zone). For a naive stored value (no Z/offset) reachable via ParseDateValue''s fallback list, display and edit-input then show different instants. The widget always emits ...Z so it only bites pre-existing/hand-authored naive data, but view and edit silently disagree. Fix: make formatDatetime parse via TZDate too so both share one interpretation; treat naive values consistently (assumed = effective zone).'
severity: significant
resolution: 'Fixed. formatDatetime now parses a naive (no Z/offset) value via TZDate (same construction path as utcISOToLocalInput) instead of native new Date(), so display and edit resolve a naive value to the SAME wall-clock. Confirmed empirically: both now agree (previously formatDatetime read naive as browser-local while the input read it via TZDate). A zoned/absolute value is unaffected. Test in format.test.ts asserts view/edit agreement for a naive value (the agreement is the contract, not a specific value).'
status: addressed
---
