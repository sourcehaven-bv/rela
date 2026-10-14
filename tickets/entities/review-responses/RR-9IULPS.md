---
id: RR-9IULPS
type: review-response
title: Create-path field verdicts stay non-monotonic
finding: 'The create gates (dataentry create, dry-run form, history recreate, entitymanager CreateEntity) read FieldVerdicts.Writable. With update as the only qualifying verb, a create-only role plus a role with a fields: block can write fewer fields on create than the create-only role alone. Counting create OR update is wrong too: a create-only role would open every field on PATCH. Make the operation explicit: create verdicts qualify block-less roles by create, update verdicts by update.'
severity: significant
resolution: Added PolicyResolver.CreateFieldVerdicts (block-less roles qualify by create). FieldWriteGate gained CheckCreateFieldWrite, used by CreateEntity; dataentry create, dry-run form and history recreate use create verdicts. Pinned by TestRoleUnion_PerDimension, TestRoleUnion_WriteGateJudgesCreateByCreate, TestCreateFieldVerdicts_RoutesToCreateCapability, TestCreateEntity_FieldGate.
status: addressed
---
