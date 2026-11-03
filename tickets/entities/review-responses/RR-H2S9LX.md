---
id: RR-H2S9LX
type: review-response
title: Missing tests for relate-failure warning, face and follow-up warning through the real promoter
finding: 'internal/appbuild/realtime_promote_test.go lacks promote_relate_failed on fs, Face reaching CreateEntity for a faced promote_to type (AC19), and promote_followup_failed. Fix: add the three cases.'
severity: minor
resolution: added appbuild tests TestPromote_RelateFailure (fs warns and keeps the entity; db builds roll back) TestPromote_FacedTarget and TestPromote_FollowUpWarning run on fs sqlite postgres and memory (6302a4b22)
status: addressed
---
