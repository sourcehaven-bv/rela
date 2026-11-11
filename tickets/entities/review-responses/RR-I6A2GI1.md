---
id: RR-I6A2GI1
type: review-response
title: 'Design: a withdrawn branch''s outputs can land after the join''s'
finding: 'join any or when: could fire while a sibling branch is between output writes, so the sibling''s set lands after the join''s outputs.'
severity: significant
resolution: 'Plan updated: the join is only tested when no live sibling has a pending mark; an arrival is recorded and the last settle tests the join.'
status: addressed
---
