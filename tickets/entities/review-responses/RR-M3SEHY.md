---
id: RR-M3SEHY
type: review-response
title: partialCascadeStore fabricates a shape the real store cannot produce
finding: Neither CL-1 nor the relation exists in the backing store so the incident set is empty and the ACL gate never runs
severity: significant
resolution: 'Fixed. The test now seeds CL-1 and the real has-checklist relation in the backing store and passes THAT relation to the double; so collectIncidentRelations returns a non-empty set and authorizeCascadeRelations genuinely runs. Also noted: this test is not coverage for the deletedRelations[i] alignment invariant - the fsstore tests own that.'
status: addressed
---

The double's `removed` holds `entity.NewRelation("REQ-1", "has-checklist",
"CL-1")`, but neither `CL-1` nor that relation exists in `backing`. So
`collectIncidentRelations` inside `deleteEntityInTx` returns empty,
`totalRelations` is 0, and `authorizeCascadeRelations` **never runs**.

The test therefore exercises a shape the real store cannot produce: a store
reporting it deleted a relation the manager just observed did not exist. A real
fsstore partial cascade always has a non-empty incident set and always passes
through the ACL gate first — the test would pass identically if that gate were
deleted.

It also cannot cover the `deletedRelations[i]` alignment invariant (the double
returns a hardcoded slice), so it must not be counted as coverage for that.

The assertions are honest; the setup is not a fair simulation. Seed `CL-1` and
the real relation so the incident set is non-empty and the gate runs.
