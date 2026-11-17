---
id: RR-P75VLW5
type: review-response
title: related used the lenient relation listing
finding: actor:related read ListRelations, which drops edges whose read fails instead of reporting it. A case could then pass on an empty result that hides a gate fault.
severity: significant
resolution: related reads ListRelationsStrict and fails the case on any listing error; an empty result now means there are no such edges.
status: addressed
---
