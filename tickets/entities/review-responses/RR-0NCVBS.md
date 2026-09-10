---
id: RR-0NCVBS
type: review-response
title: Docs and error text advised drop_entities to separate rows, which deletes the whole type
finding: Both docs/data-migration.md and the step's non-bare-target error told the operator to 'separate those rows first with drop_entities'. dropEntitiesStep is struct{Type string} — it drops the ENTIRE entity type, with no filter or predicate. An operator following that sentence to resolve 'some of my rows belong in draft' would delete every row of the type. Confidently phrased, actionable, and destructive.
severity: significant
resolution: Verified dropEntitiesStep has no filter. Removed the advice from all three places (the docs bullet, the Validate error text, and the step's Go doc comment). The docs now state explicitly that no step moves a subset of rows and that drop_entities deletes an entire entity type, so the absence of a capability is documented rather than papered over with a wrong suggestion.
status: addressed
---

Correct and the worst finding in the review by consequence, even at
"significant" — it was the only one that could destroy data, and it would have
done so to an operator doing exactly what the documentation told them.

The root of it: I wrote the guidance while reasoning about what the operator
*needs* (a way to separate rows) rather than checking what the step vocabulary
actually *offers*. That is the same failure mode as the bug itself, one level up
— asserting a remediation exists without verifying it.
