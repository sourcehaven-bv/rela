---
id: RR-ZVR1IY
type: review-response
title: Create permission check now runs against the page entity
finding: With an inverse key the relation affordance gate checks the edge's source, which is now the page entity. A user who may create the child but may not edit the page's relation gets a 403 where the backwards edge used to pass.
severity: significant
resolution: This is the correct authorization for the edge being written. Called out in the PR description.
reason: 'The gate now checks the edge that is actually written: page --relation--> new, so the principal needs permission on the page entity''s relation. The old pass was an artefact of writing the edge backwards. Called out in the PR description.'
status: wont-fix
---
