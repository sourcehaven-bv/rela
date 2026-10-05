---
id: RR-RN3YGR
type: review-response
title: selfHref keeps the bare id for the bare face while _faces[].ref spells it as ID@draft
finding: The reviewer argued the two spellings make a bookmarked /entity/policy/POL-1@draft page write to the published face because servedRef becomes the bare POL-1.
severity: significant
resolution: The bare-id fallback was removed. servedRef in EntityDetail.vue is now string | null and null until the entry loads; every address the page uses is the server's _self, face included. Script consumers return early on null and the template reads entityRef(entry) under its entry gate. A route change before the first load no longer pins the route id for an autosave flush. Covered by EntityDetail.accept.test.ts. TKT-PB9VDL tracks splitting the loaded view into a child so the guards go away.
status: addressed
---
