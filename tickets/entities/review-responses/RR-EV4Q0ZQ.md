---
id: RR-EV4Q0ZQ
type: review-response
title: 'Code: face toggle could switch to an unreadable world'
finding: FaceToggle chose the world by chain head only. A reader with a face grant but no world grant landed on a denied world that serves nothing.
severity: significant
resolution: FaceToggle switches world only when the target world is readable; otherwise it keeps the current query. Covered by FaceToggle.test.ts.
status: addressed
---
