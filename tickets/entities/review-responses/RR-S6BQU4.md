---
id: RR-S6BQU4
type: review-response
title: mentioned-by 404 must reuse entity GET lookup
finding: A separate readability check could make hidden and absent B differ.
severity: nit
resolution: The handler resolves the target through readAddressedOr404 (visibleReader.address), the same gate as the entity GET. TestMentionedBy_HiddenTargetIs404 compares the hidden and missing response bodies with the id masked.
status: addressed
---
