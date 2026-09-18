---
id: RR-W5T5Z7
type: review-response
title: 'Test hygiene: weak dedupe assertion, shadowed schema seed, unmocked API default'
finding: Three test-quality points. (1) The dedupe test asserted only length===1, which a dedupe that dropped both chips would also satisfy. (2) mountWithPaging re-registered the relation type seedSchema had just set, two writes to one store key with the second silently shadowing the first. (3) The file-level vi.mock('@/api') left getEntityRelations returning undefined for every pre-existing describe, safe only because those mount pickers without an entityId — adding one would crash them in .filter.
severity: minor
resolution: (1) Added an assertion that the surviving chip is the right one. (2) Commented why the helper re-registers, so the shadowing reads as intent. (3) Added a file-level beforeEach defaulting the mock to [], removing the trap. Also hoisted knownById.value out of the buildOutgoingTypes loop, and added a removeEntity test covering the same rebuild path as selectEntity.
status: addressed
---
