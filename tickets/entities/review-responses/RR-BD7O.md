---
id: RR-BD7O
type: review-response
title: 'Test gap: task items survival through transformations'
finding: No test that task item table shape survives shift_headers/set_min_header_level/concat. shiftNodeHeaders and deepCopyNode use ForEach which should preserve them, but no test pins this. A future optimization to skip non-heading types could silently break task lists.
severity: significant
resolution: 'Added TestMdTaskListSurvivesShiftHeaders: parses content with header + task list, applies shift_headers(1), and verifies the task items survive intact through the deep-copy walker.'
status: addressed
---
