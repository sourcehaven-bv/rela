---
id: RR-QVPW
type: review-response
title: No test for 3+ trailing spaces or hard break at EOP
finding: Both are edge cases CommonMark spells out explicitly; trivial to add while touching the file.
severity: nit
reason: Out of scope for this xs render-config flip. These edge cases test marked.js parser behaviour, not our breaks:false change — they would belong to a marked.js parity-test suite, not this ticket. Three regression tests (soft-break absence, hard-break presence with position pinning, paragraph split on blank line) cover the actual change.
status: wont-fix
---
