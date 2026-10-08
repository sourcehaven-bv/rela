---
id: RR-1F1UXX
type: review-response
title: Gated production wiring untested
finding: The sqlite test had no acl.yaml, so the gated WithHistory wiring could be removed with no failure.
severity: significant
resolution: The policy test runs through ScheduledLuaWriteDeps with acl.yaml (mutation-checked against removing WithHistory); dataentry forwarding has its own test.
status: addressed
---
