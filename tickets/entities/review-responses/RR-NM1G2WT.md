---
id: RR-NM1G2WT
type: review-response
title: 'A change branch without when: moves on any edit'
finding: Under the plan every ordinary write fires change; a branch without when (e.g. roles only) would move on any edit by a role holder, changing the meaning of existing flows.
severity: significant
resolution: 'An ordinary write takes only change branches with a when: that reads a property the write changed. Step completion is unchanged.'
status: addressed
---
