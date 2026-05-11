---
id: RR-QH41
type: review-response
title: 'Edge-case test gap: target type without create_form'
finding: 'Pre-change resolver had divergent branches: AddInfo only when len(targets) > 0, LinkInfo whenever len(candidateTypes) > 0. A target type without create_form emitted only linkInfo. Post-change neither emits. Adding a sub-test for the ''no create form'' shape would catch a future regression where someone re-adds linkInfo only.'
severity: minor
resolution: The 'outgoing-cards-no-form' sub-test in the restructured TestV1Views_NoAddOrLinkInfoOnSections explicitly covers the edge case where the target type has no create_form configured (which previously emitted only linkInfo). Asserts both addInfo and linkInfo are absent.
status: addressed
---
