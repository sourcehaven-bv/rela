---
id: RR-4YKPR85
type: review-response
title: entity_refs ids are not validated like content ids
finding: content ids pass entity.ValidateID through mentions.All but ids accepted any non-empty string so an address such as TKT-1@draft silently matched nothing.
severity: nit
resolution: Each opts.ids entry goes through entity.ValidateID and raises on failure. TestEntityRefs_IDs covers TKT-1@draft.
status: addressed
---
