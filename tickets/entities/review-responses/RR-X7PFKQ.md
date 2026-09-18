---
id: RR-X7PFKQ
type: review-response
title: A row of an undefined type was reported as bare-row-on-faced-type with an impossible remedy
finding: 'faceDeclared returned false when GetEntityDef missed. For a BARE row that falsehood flowed into the p.IsDefault() branch and got the faced-type finding, so a row whose entity type the metamodel does not define at all was told it was ''stored at the bare id on a type that declares faces:'' and handed a migrate_face remedy. migrateFaceStep.Validate proves its mapping total against the type''s declared projections, which do not exist for an undefined type, so the operator''s migration would refuse to parse. A finding naming the wrong cause and prescribing a remedy that provably fails. Introduced by this commit''s reordering: the old code returned true early for the zero face, masking it.'
severity: critical
resolution: 'Reproduced first (a `phantomtype` row reported under bare-row-on-faced-type alongside real ones). Replaced the boolean faceDeclared with faceStatusOf returning a faceStatus enum (faceOK / faceUnknownType / faceUndeclared / faceBareOnFaced), so the caller knows WHICH fault it holds instead of guessing. An undefined type now reports under its own code `unknown-entity-type`, naming the type, with a remedy that can actually apply (declare the type or remove the rows). Mutation-verified: collapsing faceUnknownType into faceBareOnFaced fails the test.'
status: addressed
---

The reviewer's framing was the useful part: `faceDeclared` conflated "type
declares this face" with "type exists", and a bool cannot carry the difference.
That conflation also affected the *named*-face case, which reported a `draft`
row on an undefined type as `undeclared-face` — same wrong cause, quieter
symptom. Both are fixed by the same enum.
