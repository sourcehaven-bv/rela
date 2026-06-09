---
id: RR-FD1E
type: review-response
title: 'Round 1 #5-10: minor concerns + leverage'
finding: |
  - #5 (minor): Pointer-to-map for _props isn't needed (plain map + omitempty); only _fields needs the idiom.
  - #6 (minor): copyTypedProperties shallow copy is correct; deep is overkill since the response is JSON-marshaled immediately.
  - #7 (minor): AC 3's "mutate the returned map; ensure next GET unmutated" test asserts the wrong thing — JSON marshaling already prevents shared mutable state. Drop it or rewrite to assert the converter doesn't share the entity's underlying property map pointer.
  - #8 (already covered by RR-FD1B): hidden affordance interaction.
  - #10 (leverage): alternative (b) reasoning is wrong — buildSections already takes ctx and threads it. Storing _fields on SectionEntityData (unexported) is actually cleaner than threading the entity to the wire converter.
severity: minor
status: addressed
resolution: |
  - #5: Plan AC 1 updated — _props is `map[string]any` with `omitempty`; only FieldAffordances stays pointer-to-map.
  - #6: Plan keeps shallow copy; rationale added (the defensive copy guards against future maintainers aliasing the entity's property map into a long-lived response, not against the JSON-side client).
  - #7: AC 3 mutate-and-refetch test DROPPED. Replaced with a static assertion that the wire converter's Props field is a fresh map, not the entity's underlying reference (single-line `reflect.ValueOf` check during the existing test).
  - #10 ADOPTED: reverse alternative-rejection. Compute _fields inside buildSections (which already has ctx) and store on SectionEntityData. Drop the `entity *entity.Entity` back-reference field. Wire converter just reads the precomputed Fields. Cleaner package boundaries; same total work.
---
