---
id: RR-0HB6OX
type: review-response
title: faceCandidateProperty's single-enum fallback drafts a skeleton keyed on an unrelated property
finding: 'The status/state name match is sound, but the single-candidate fallback is not: a type whose only enum is priority, severity or language gets a skeleton keyed on it, pre-filled and wrapped in prose asking whether each priority value ''belongs in published'' — a question with no meaning, which trains the operator to accept without thinking. The function''s own doc comment names this hazard (''a wrong guess produces a plausible-looking skeleton keyed on the wrong property, harder to notice than no skeleton at all'') and the code then walks into it. Exactly one candidate is evidence of a small schema, not of relevance.'
severity: significant
resolution: Kept the fallback but made it declare itself. faceCandidateProperty now returns a byElimination flag; when set, the drafted comment says the property is the type's only enum, was chosen by elimination, may have nothing to do with content state, and offers both alternatives (key on a different property, or drop property/mapping for the whole-type form). Removing the fallback outright was the other option; offering a labelled weak guess beats offering nothing, as long as the label is there.
status: addressed
---

Correct. The comment describing the hazard was written before the fallback, and
I did not re-read it against what the code ended up doing.

Chose to label rather than remove because the alternative leaves an operator
with a type that has one enum and no skeleton at all — and the whole-type form
now gives them a real out either way. The label carries the weight: a guess
presented as a guess is useful, a guess presented as a recommendation is the
trap the comment warned about.
