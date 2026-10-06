---
id: RR-QZZIMN
type: review-response
title: After a body conflict the next keystroke overwrites the whole body
finding: On a content conflict contentBase becomes theirs while the editor keeps the user's full text; the next edit passes the precondition and reverts non-conflicting hunks of the other user.
severity: significant
resolution: 'mergeText now returns oursInConflicts on a failed merge: their clean hunks with our lines in each conflicting region. After a body conflict the editor shows that text and the base becomes their body, so the next save overwrites only the conflicting regions. Docs updated. Test: ''shows their clean hunks after a body conflict'' (mutation-verified).'
status: addressed
---
