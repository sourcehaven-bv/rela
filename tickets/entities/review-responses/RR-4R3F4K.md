---
id: RR-4R3F4K
type: review-response
title: The rename guard broke `rela migrate gen` for any faced type rename
finding: The new renameEntityTypeStep.Validate check made the generator emit a migration that fails its own validation. possible_entity_type_rename is guessed by propertyShapesSimilar, which compares PROPERTIES only, so it pairs a faced `task` with a faceless `job`; the generator emits a live rename_entity_type; Generate's round-trip self-check then rejects it and returns nil with 'generated draft does not parse (generator bug)'. The operator gets NO FILE AT ALL and an error blaming the tool, for a schema change they are entitled to make. Worse than the bug being fixed.
severity: critical
resolution: 'Reproduced with a probe before acting (draft=false, error text confirmed verbatim). draftActiveStep now calls facesNotIn and, when the face sets diverge, emits the rename COMMENTED with a TODO naming the orphaned faces and the remedy steps (rename_face / migrate_face) — the pattern the generator already uses for map_values. The draft parses, stays useful, and says what must happen first. Mutation-verified: removing the branch reproduces the exact ''generator bug'' failure an operator would have hit.'
status: addressed
---

This is the finding worth the whole review. The guard itself was right and
narrow; its interaction with the generator was not something the guard's own
tests could see, because `generate_test.go` had no faced-rename case.

The failure mode is the kind that surfaces to an operator mid-migration, with an
error message that sends them looking for a tool bug rather than at their own
schema.
