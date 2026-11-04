---
id: RR-R6T79M9
type: review-response
title: 'POC example assumes in: config the POC does not have'
finding: The POC realtime.yaml has no default_in or in:, so in plan-review the owner may post every owner type; the example was wrong. Many POC one-click types have optional note or reason fields that lose their web path.
severity: significant
resolution: 'Example corrected to name the planned POC config (default_in: keyed) and today''s behaviour; the optional-field loss is stated as the main trade-off (open choice 4) with backlog TKT-HG4PO94; kanban-busy e2e added to the files list.'
status: addressed
---
