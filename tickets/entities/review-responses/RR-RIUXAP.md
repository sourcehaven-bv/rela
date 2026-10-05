---
id: RR-RIUXAP
type: review-response
title: 'Design: fixme specs must assert the fixed behaviour'
finding: A test.fixme that asserts today's broken output would pass trivially or assert the bug when flipped on. The plan did not say which behaviour the fixme bodies encode.
severity: minor
resolution: Every fixme body asserts the Expected section of its bug; the plan and a header comment in faces-backlog.spec.ts say so.
status: addressed
---

A test.fixme that asserts today's broken output would pass trivially or assert
the bug when flipped on. The plan did not say which behaviour the fixme bodies
encode.
