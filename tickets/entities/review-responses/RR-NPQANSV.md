---
id: RR-NPQANSV
type: review-response
title: 'Design: if_exists replace is an unchecked delete'
finding: 'A judged trigger that already has if_exists: replace can be pointed at other records and deletes them without an ACL check.'
severity: significant
resolution: 'The checker refuses an added or changed trigger, or a trigger the cascade reaches, whose create output has if_exists: replace.'
status: addressed
---
