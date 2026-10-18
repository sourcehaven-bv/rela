---
id: RR-O7PAIE
type: review-response
title: Promoter wired to the assembly Manager bypasses the acl.yaml field gate
finding: Services.entityManager is not field-gated (entitymanager.FieldGated sets fieldGate=true; fieldGated(m) is false on the base handle). Wiring managerPromoter from "the assembly's Manager" (C1/C4) lets a caller write a field they may not author through a promote mapping. Wire appbuild.FieldGatedEntityManager and pin it with the AC16 test through the real wiring rather than a hand-built Manager.
severity: critical
resolution: The promoter is wired to appbuild.FieldGatedEntityManager. AC16 runs through the real wiring, not a hand-built Manager.
status: addressed
---
