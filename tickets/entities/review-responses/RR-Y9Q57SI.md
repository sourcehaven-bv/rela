---
id: RR-Y9Q57SI
type: review-response
title: 'AC4 coverage: mail sent from a Lua action'
finding: expect_mail is tested with mail from an automation, not from a Lua action.
severity: minor
reason: 'Deferred to TKT-E2IMTMG: both paths use the same jobs queue and memory sender, so the suite adds the Lua case.'
status: deferred
---
