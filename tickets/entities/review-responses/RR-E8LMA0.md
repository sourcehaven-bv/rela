---
id: RR-E8LMA0
type: review-response
title: Change pushed RelationPicker.vue over the max-lines warning threshold
finding: 'The review asserted the max-lines warning on RelationPicker.vue was pre-existing and not introduced by this change. It was not: stashing the change and re-linting the original file produced no warning, while the changed file reported 510 (limit 500, counted with skipBlankLines and skipComments — so this is real code, and trimming comment prose does not help).'
severity: minor
resolution: Rejected the reviewer's premise after checking it, then fixed the underlying issue rather than accepting a warning. Extracted the pure logic (missingIds, mergeResolvedLinks, indexKnownEntities, resolveSelected) into outOfPageLinks.ts, which clears the threshold and makes the folding rules directly unit-testable — 10 new tests in outOfPageLinks.test.ts covering merge-not-replace, untyped-edge rejection, candidate precedence, ordering and dedupe.
status: addressed
---

Recorded because the correction matters more than the finding: an incorrect
"pre-existing" attribution is the kind of claim that, taken at face value,
licenses shipping a regression. The check is cheap — stash, lint, pop.
