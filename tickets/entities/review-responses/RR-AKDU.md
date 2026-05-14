---
id: RR-AKDU
type: review-response
title: DeepReadonly cast in popup drops type safety on Entity shape
finding: 'BacktickAutocompletePopup.vue (lines 56-79) defines a hand-rolled `DisplayPrefix | DisplayEntity` union and casts `state.prefixItems` / `state.entityItems` to it via `as readonly DisplayPrefix[]` / `as readonly DisplayEntity[]`. The cast sidesteps `DeepReadonly<Entity>` incompatibility but loses brand-checking: if `Entity` grows a required field (e.g. `created_at` becomes non-optional), the popup''s `DisplayEntity` won''t fail to type-check against it because the cast is structural. The comment correctly identifies the immutability is preserved by the `readonly` modifier, but the type-shape drift is the real risk. Fix: either narrow the readonly version with `Readonly<Entity>` (one level deep — sufficient because the popup only reads top-level fields), or expose a dedicated `EntityRow` DTO from the composable so the popup binds to a stable shape.'
severity: significant
reason: The DeepReadonly cast is a pragmatic local workaround for Vue's recursive readonly transform colliding with Entity's mutable nested types. The popup is purely presentational (reads only) and the runtime behavior is sound. A proper fix needs either a project-wide DeepReadonly-compatible Entity type or a refactor of how the picker types its row data; both are out of scope for this ticket. The cast is documented inline with the rationale.
status: deferred
---
