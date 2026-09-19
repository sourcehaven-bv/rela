---
id: RR-N95ULU
type: review-response
title: The 14-gate regression tests exercise a hand-rolled fixture, not the production adapter
finding: 'Code review. internal/validation/relation_constraint_test.go uses testGraph, a stand-in, because validationgraph imports validation and an internal test cannot close that cycle. The rationale is sound, but the consequence is that the pre-existing gate tests validate a fixture that does not behave like production: it filters in Go rather than through store.RelationQuery and hard-errors on DirectionIncoming. Concretely, the eager-read regression (RR-X7JUMF) was invisible to every test in that package, because the fixture and the adapter independently made the same wrong choice. The adapter''s own tests never counted reads.'
severity: significant
resolution: Added TestRelatedEntities_NoFarReadsWhenNotRequested in the adapter's package with a counting reader, which is the storetest.Counting-shaped budget assertion CLAUDE.md mandates for new read paths. Also note relation_direction_test.go is an EXTERNAL test (package validation_test) that uses the REAL adapter, so the direction and target_type behaviour is exercised end to end rather than against a stand-in; the internal-package fixture now honours resolveFar so it cannot drift on that axis either. The cycle prevents removing testGraph outright, so the mitigation is coverage on both sides rather than a single fixture.
status: addressed
---
