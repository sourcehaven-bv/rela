---
id: RR-1N919GH
type: review-response
title: Go test depends on tickets project data
finding: TestTicketsTriggersCovered reads tickets/flows.yaml and the Lua suite from the repo, coupling a package test to project data.
severity: minor
reason: 'The coupling is the purpose: the test fails when a trigger is added to tickets/flows.yaml without a case.'
status: wont-fix
---
