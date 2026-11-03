---
id: RR-R9KPDX
type: review-response
title: Offer allowed can disagree with Convert
finding: Offers skips the hidden copied-field check and judges the edge gate on the raw entity while Convert judges the redacted row, so an offer can say allowed and Convert answer 403.
severity: minor
resolution: Offers now read the source through the copy reader once, refuse a recipe that copies a hidden field, and judge the edge gate on the redacted row, as Convert does. Pinned by TestTaskItemOffers_JudgeTheSeenSource.
status: addressed
---
