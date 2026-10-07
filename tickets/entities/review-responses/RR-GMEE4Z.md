---
id: RR-GMEE4Z
type: review-response
title: Interaction with the cascade flag is undefined
finding: DeleteEntity takes cascade. CLI, MCP, Lua and CalDAV default it to false and today refuse with ErrHasRelations when edges exist; the SPA soft delete passes true. The plan says deleting a parent deletes its children without saying whether that happens on a non-cascading delete.
severity: significant
resolution: 'Plan: children are deleted only on cascade=true. With cascade=false the owning edges are ordinary edges and ErrHasRelations applies unchanged (AC4a). SPA soft delete already cascades.'
status: addressed
---
