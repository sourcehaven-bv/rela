---
id: RR-4EFIW0
type: review-response
title: 'Q5: confirm/style on an entry without a button is ambiguous for the type button'
finding: 'The type button aggregates every entry without a button, so entries that disagree on confirm: or style: have no single answer. Recommend button.confirm and button.style inside button: for presets, with a type-level key (or one derived as now) for the type button. Alternatively, state the merge rule, for example ''confirm if any entry confirms, danger if any is danger''. The current wording leaves TKT-TXZZZ2 to rediscover the problem.'
severity: minor
resolution: 'Q5 now places confirm and style inside button: for presets, with a type-level key for the shared type button, and TKT-TXZZZ2''s note is to be updated to match.'
status: addressed
---
