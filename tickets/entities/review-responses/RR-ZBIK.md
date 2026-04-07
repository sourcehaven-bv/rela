---
id: RR-ZBIK
type: review-response
title: Multi-block list items concatenate text with no separator
finding: extractListItems concatenates text from all TextBlock/Paragraph children with no separator. Multi-paragraph items produce 'firstsecond' with no space. The 'multi-block does not crash' test only checks non-empty output, not content. Now hidden behind a 'text' field that scripts treat as authoritative.
severity: critical
resolution: extractListItems now captures only the FIRST text block of each list item (with break statement), instead of concatenating all text blocks. This matches the GFM task list spec (checkbox must be in the first text block). Documented in function comment. TestMdTaskListMultiBlockItem covers continuation-line items.
status: addressed
---
