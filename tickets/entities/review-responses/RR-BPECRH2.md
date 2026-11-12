---
id: RR-BPECRH2
type: review-response
title: A parked job branch holds every join
finding: 'settle skipped the join whenever a branch was parked, so join any, a when: join or a branch reaching a final state waited for an unrelated job.'
severity: significant
resolution: settle always asks the join; a parked branch has not arrived, so only a join that needs it waits. TestJob_ParkedBranchDoesNotHoldAJoinThatDoesNotNeedIt; the guide says which joins wait.
status: addressed
---
