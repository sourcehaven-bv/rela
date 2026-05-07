---
id: RR-RO22H
type: review-response
title: Type names EntityType vs EntityTypeConfig too easy to confuse
finding: 'frontend/src/types/ already exports EntityType (metamodel). Adding EntityTypeConfig next to it invites import errors. Use a more distinctive name: EntityViewConfig, EntityTypeRouting, or co-locate as EntityType.routing?: { detailView?: string }.'
severity: minor
resolution: Frontend type is named EntityViewConfig (not EntityTypeConfig). Schema store ref is named entityViewConfigs (not entityTypes -- the metamodel one). Distinct from existing EntityType (metamodel) to prevent import confusion.
status: addressed
---
