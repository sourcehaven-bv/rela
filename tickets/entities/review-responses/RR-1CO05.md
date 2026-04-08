---
id: RR-1CO05
type: review-response
title: Mutation handlers mutate live graph nodes before UpdateEntity
finding: 'handleAPIUpdateEntity, handleV1UpdateEntity, handleToggleCheckbox all do `entity := a.Graph().GetNode(id); oldEntity := entity.Clone(); entity.Properties[k]=v; entity.Content=...; ws.UpdateEntity(entity, oldEntity)`. The middle steps mutate the LIVE graph node''s Properties map and Content field while concurrent lock-free readers can be reading them. writeMu only serializes writers vs writers, not writers vs readers. The refactor didn''t introduce this race (pre-existing entity-property unsynchronized access) but removed the token RLock that previously masked it. Fix: clone the entity TWICE (once for oldEntity, once for the working copy), mutate the working copy, pass it to UpdateEntity which atomically swaps via Graph.mu.'
severity: significant
reason: This is a pre-existing race in the entity-property map model that predates TKT-252Y and the App.mu refactor. It is documented in the TestConcurrentReloadStateSnapshot scope notes from TKT-252Y and again in TKT-Z7HL. Fixing it requires a clone-before-mutate pattern in every mutation handler (not just the dataentry ones — the same pattern exists in CLI and MCP). Tracked as a separate follow-up ticket.
status: deferred
---
