---
id: RR-B1JI4C
type: review-response
title: Display-mode widget test asserts existence, not zone-correct output
finding: 'widgets.test.ts display-mode test sets the zone after mount and only asserts span.display-value exists and input absent; it never checks the rendered text is zone-correct, so the formatDatetime display path is untested for correctness. Fix: mount with a known non-UTC zone (e.g. America/New_York) and assert the formatted text contains the expected local time.'
severity: minor
resolution: Fixed. The display-mode widget test now sets zone America/New_York BEFORE mount and asserts the rendered text contains '8:30' (zone-correct), not just element existence. Added a dedicated formatDatetime describe block in format.test.ts with zone-correctness, naive-consistency, null-on-unparseable, and null-on-bad-tz cases.
status: addressed
---
