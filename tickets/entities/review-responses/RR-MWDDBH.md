---
id: RR-MWDDBH
type: review-response
title: validateStepOrder is per-file and catches only drop_property, but its comment claims generality
finding: 'Three parts. (1) The check runs inside ParseFile so it is per-file; a drop_property in file 1 and a confirm_face in file 2 is not caught (Validate''s from-schema check catches it incidentally, which is accidental coverage from a different mechanism). (2) Only dropPropertyStep is matched — a lua step removing the property, or a map_values remapping every value, produce the same ''confirms nothing'' outcome uncaught; map_values is the interesting one, since it makes every row report as uncovered. (3) The rule is arguably the wrong shape: the step writes nothing, so a bad order degrades a report rather than data, yet it refuses a parse while genuinely dangerous orderings pass.'
severity: minor
resolution: 'Corrected the doc comment, which was the part that actually misled: it no longer claims to see constraints ''no single step can see'' in general. Left the check itself narrow. Widening it to lua would require reading the script, and to map_values would mean modelling value flow across steps — both are a value-provenance feature rather than an ordering check, and the map_values case now produces an accurate note rather than a wrong claim about the data (see RR-OHKFRO).'
status: addressed
---

All three sub-points are right; the fix taken is the comment, not the check.

On (3), the reviewer's suggestion to widen rather than keep it narrow-and-strict
is reasonable, but widening to `lua` means reading the script and widening to
`map_values` means tracking value flow across steps. That is a different feature
with its own design, not a tweak to an ordering check. The concrete harm from
the `map_values` case was the misleading note, and that is fixed at the note.

Kept the parse refusal for the `drop_property` case: it costs an operator one
edit and the alternative is a step that silently reports on nothing.
