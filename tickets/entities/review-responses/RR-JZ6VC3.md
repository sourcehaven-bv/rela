---
id: RR-JZ6VC3
type: review-response
title: Every relation write reloads every open detail panel without debounce
finding: Each relation write broadcast entity:changed for both end types. Every open detail panel of those types reloaded once per event with no debounce.
severity: minor
resolution: 'Fixed in 4884f92a7 with a 250 ms debounce, and a 1 s max wait in round 2 (82f0f9a0d). Section forms keep pending edits across a reload: the SectionEditForm initialValues watcher merges through autoSave.mergeServerResponse. Covered by EntityDetail.events.test.ts.'
status: addressed
---

Review finding R1-10.
