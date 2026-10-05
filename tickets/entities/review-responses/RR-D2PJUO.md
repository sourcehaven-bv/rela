---
id: RR-D2PJUO
type: review-response
title: facedTest and facedPgTest were near copies
finding: testProject, facedApi and the serverUrl teardown were duplicated between the two variants.
severity: significant
resolution: Hoisted shared bodies (facedProject, serveFaced, facedApiFixture); each variant now only supplies its env and log name. Base-fixture options were not added to avoid touching the shared fixture for every spec.
status: addressed
---

testProject, facedApi and the serverUrl teardown were duplicated between the two
variants.
