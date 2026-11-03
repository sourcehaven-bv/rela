---
id: RR-JTUMFP
type: review-response
title: Dead null check in submitCreate
finding: ChannelsSection.vue:107 checks `!a` on anchor.value, which is a computed object and never null. Drop the check.
severity: nit
resolution: The dead anchor null check in submitCreate is gone.
status: addressed
---
