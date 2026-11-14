---
id: RR-ZWWATR9
type: review-response
title: Store-side count makes the BUG-BINAR2 hidden-field oracle cheaper
finding: 'rela-security-reviewer: with meta.total counting every match, a filter on a hidden property reveals in one limit=1 request how many rows hold a guessed value. The same disclosure was already available by reading every row.'
severity: minor
reason: Same channel as BUG-BINAR2, which covers structured filters on hidden fields for both paths; its fix must also cover the count and the position.
status: deferred
---
