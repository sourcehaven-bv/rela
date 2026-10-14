---
id: RR-9FKZ9H
type: review-response
title: Load-time visibility lint scans only lists
finding: 'listVisibilityConflicts scans only cfg.Lists; kanban filter controls, calendar date properties and where clauses, sidebar entities sort and client-ceiling visible:/redact: are not checked.'
severity: significant
resolution: 'configuredFieldUses now collects list sort/group_by/filters/filter controls, kanban filter controls, calendar source date and where properties, and sidebar entities: sort (EffectiveNavSort). baselinesHidingField adds a warning for a client baseline whose visible: omits or redact: names the property. TestListVisibilityConflicts covers every surface and the three baseline cases.'
status: addressed
---
