---
id: RR-K6FC0ML
type: review-response
title: The cutover can lose a job an older process inserts
finding: FOR UPDATE NOWAIT locks only existing rows. An idle older process could insert a job after the SELECT and DROP TABLE would delete it unmoved.
severity: minor
resolution: 'Addressed: the cutover takes LOCK TABLE neoq_jobs IN ACCESS EXCLUSIVE MODE NOWAIT before reading. That also conflicts with a running worker''s row lock.'
status: addressed
---
