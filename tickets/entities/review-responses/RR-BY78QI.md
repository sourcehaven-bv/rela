---
id: RR-BY78QI
type: review-response
title: Restore denied-write audit record untested
finding: TestTransition_RecreateEntity_GuardedStateIs403 used audit.Nop, so nothing asserted that the denial is audited.
severity: minor
resolution: The test now records to audit.NewMemory and asserts one denied-write record naming the entity.
status: addressed
---
