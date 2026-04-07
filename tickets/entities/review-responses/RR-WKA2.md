---
id: RR-WKA2
type: review-response
title: 'Test gap: empty task text parse case'
finding: No test for parsing `- [ ]\n` or `- [x]\n` (checkbox with no text). Constructor side is tested but not the parser side.
severity: significant
resolution: 'Added TestMdTaskListEmptyText: parses ''- [x] \n- [ ] \n'' and verifies both items have task=true, correct checked state, and text=''''.'
status: addressed
---
