---
id: RR-CNM4HB
type: review-response
title: 'Proposed adapter home is unbuildable: analysis cannot import validator'
finding: The plan put the store-backed adapter in internal/validator, 'with appbuild and analysis wiring it'. But .go-arch-lint.yml analysis.mayDependOn is [frontmatter, lua, metamodel, project, schema, storage, store, tracer, validation] — validator is absent — and analysis.newValidationService is the second entry point into validation.Service. So analysis cannot construct the adapter, while the ticket asserts just ci (including arch-lint) green. lua being the only package both validator and analysis can reach is precisely why OutgoingRelations ended up on ReadDeps in the first place.
severity: significant
resolution: 'Verified against .go-arch-lint.yml. User chose a new leaf package both can import. Plan and ticket now specify internal/validationgraph (working name) with mayDependOn: [entity, metamodel, store], added to validator and analysis. internal/validation does NOT depend on it — it declares the interface and receives an implementation — so the ''validation must not import store'' criterion still holds. Rejected alternatives recorded: adding validator to analysis.mayDependOn (new edge between peers), and widening ReadDeps in place (keeps the shape the ticket exists to correct).'
status: addressed
---
