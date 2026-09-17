---
id: RR-L7RANS
type: review-response
title: Stale comment on handleEntityCreated would lead to re-introducing the bug
finding: RelationPicker.vue handleEntityCreated carried the comment "loadCandidates fetches only the first 100 per type", which the fix made false. The candidates.value.push(entity) it justifies is still REQUIRED (the entity postdates the fetch, so it is in no page), but a reader who greps, finds fetchAllList, and concludes the comment is stale would delete the push along with it -- silently reintroducing BUG-HOB9BR on the inline-create path.
severity: critical
resolution: Rewrote the comment to state the true reason (the entity did not exist when loadCandidates ran, so it is in none of the pages) and added an explicit "do not drop this on the grounds that candidates are complete" warning naming the failure it would cause.
status: addressed
---
