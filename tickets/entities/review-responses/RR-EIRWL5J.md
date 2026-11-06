---
id: RR-EIRWL5J
type: review-response
title: Create outputs may copy subject values to a type with a wider read audience
finding: 'The template check covers visible: fields, not row-level audience of the created type.'
severity: significant
resolution: Documented as an operator choice, like automation create_entity interpolation, in the Flows guide; the runtime writes through entitymanager as run_as. A row-audience subset check cannot be decided at load because row grants depend on role_relations.
status: addressed
---
