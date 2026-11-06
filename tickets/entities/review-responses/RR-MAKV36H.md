---
id: RR-MAKV36H
type: review-response
title: A grant whose when fails still lifts the meta allowlist
finding: relationVerdicts lifted the allowlist for any role that held update and declared no meta fields for the type. A grant whose when failed, or a relations block that did not list the type, still lifted it.
severity: significant
resolution: Only a role with no relations block for the source type, or a passing grant for the type without fields, lifts the allowlist. TestRelationMeta_FieldsGrantIsAnAllowlist adds pia, whose failing grant must not lift it.
status: addressed
---
