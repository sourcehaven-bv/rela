---
id: RR-IFKGBYU
type: review-response
title: searchAllEntities doc omits delete skips
finding: Offset paging can skip a row when an earlier row is deleted between pages; the helper doc did not say so.
severity: minor
resolution: The doc comment now states that a concurrent delete can skip a row and that free text stops at the cap.
status: addressed
---
