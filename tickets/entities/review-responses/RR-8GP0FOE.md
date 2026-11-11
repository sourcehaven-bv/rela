---
id: RR-8GP0FOE
type: review-response
title: Run ordering degrades silently when the join is nil
finding: joinFlowRunsFor returns nil without a log line when the run store or the store is not the database tier.
severity: nit
reason: The race tests on sqlite and postgres go through the production wiring (buildFlowEngine and openFlowEdits), so a wiring change that drops the join fails them.
status: wont-fix
---
