---
id: RR-DOCA06
type: review-response
title: claim lists silently dropped non-strings and read a scalar as 'assert empty'
finding: fieldStringSlice skipped non-LString elements, so exactly={"a", 42} asserted a smaller set than written. Worse, a scalar exactly="a" read as present-but-empty via hasField, silently becoming 'assert this type is empty' — the opposite of what the author wrote. Both coercions weaken the claim, which is the wrong direction to fail.
severity: significant
resolution: Assertions use a strict claimList that refuses a non-table value and any non-string element. fieldStringSlice is unchanged for the existing non-assertion verbs.
status: addressed
---
