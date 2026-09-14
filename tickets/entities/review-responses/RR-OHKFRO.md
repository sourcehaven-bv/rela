---
id: RR-OHKFRO
type: review-response
title: List-typed enums passed validation then reported every row as uncovered
finding: enumValuesIn ignored PropertyShape.List, so a list-typed enum passed Validate. At runtime Run reads the property with a string type assertion, which fails for every []any value, so EVERY row landed in the unaccounted bucket and was reported as outside the declared set. The operator would be told their entire dataset is invalid when it is not — the step simply cannot read a list. The comment above that branch asserted 'pre-existing invalid data', a cause the code is not entitled to infer.
severity: significant
resolution: Validate now rejects List:true with a message explaining that a row holding several values has no single face, pointing at the whole-type form instead. faceCandidateProperty also skips list-typed properties so the generator never drafts one. The Run comment now describes what was observed (the mapping did not cover these rows) rather than asserting why, since an earlier map_values or lua step in the same file can produce the same symptom. Pinned by TestConfirmFace_RefusesListTypedKey.
status: addressed
---

Correct on both halves, and the comment half is the more useful finding. The
code was defensible; the comment asserting a *cause* it could not know would
have sent the next person debugging a flood of these notes to inspect the data
instead of the step.

The reviewer's point generalizes: `Validate` proves the mapping total over the
DECLARED value set, which is a statement about the schema, not about what rows
carry. Any inference from "uncovered at runtime" to "invalid data" skips over
every step that can change values in between.
