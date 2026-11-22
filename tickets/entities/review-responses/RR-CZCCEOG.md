---
id: RR-CZCCEOG
type: review-response
title: "parkFailed copies storeJob"
finding: "parkFailed repeated the pending, instance and lost-run handling of storeJob line for line."
severity: significant
reason: "storeJob and parkFailed now share storeJobStep, which takes an optional log step."
status: addressed
---
