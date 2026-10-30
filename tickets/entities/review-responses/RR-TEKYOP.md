---
id: RR-TEKYOP
type: review-response
title: liveSub keeps growing per-shape fields
finding: 'internal/dataentry/live.go:181-200: liveSub now carries presenceID, channel, inbox, busy, wake (*inboxWake), busyWake, topic and reader. Each shape uses a subset, and selects, applyTopic, fallback, collect and safetyDue each branch on the flags in their own order. A later shape will add one more flag and one more branch in five places. Fix (follow-up is fine): give liveSub one shape-specific value, for example an interface with selects(f), apply(res), fallbackFrame(topic) and safetyEligible(), that the presence, channel, inbox and busy shapes implement.'
severity: minor
reason: A per-shape interface over presence, channel, inbox and busy changes selects, applyTopic, fallback, collect and safetyDue for all four shapes. That is a refactor of the whole hub, outside this ticket. The reviewer accepts a follow-up. Follow-up TKT-S3YYGM.
status: wont-fix
---
