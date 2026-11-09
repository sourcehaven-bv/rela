---
id: RR-BDBOB01
type: review-response
title: 'Design: paging contract easy to misuse'
finding: lo is inclusive and the client must resume after last; NUL is refused in bounds so last+NUL is impossible; more can be true with zero elements.
severity: significant
resolution: A range takes either lo (inclusive) or after (exclusive); the response's last feeds after. hi absent means unbounded. Docs say to loop on more and never on the element count.
status: addressed
---
