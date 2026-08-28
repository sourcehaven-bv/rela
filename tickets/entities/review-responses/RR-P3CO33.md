---
id: RR-P3CO33
type: review-response
title: Adding defineEmits to DynamicForm changes attribute fallthrough; "byte-identical" is overstated
finding: The risk table claims an additive prop leaves every existing mount "byte-identical in behaviour". DynamicForm has no defineEmits today; declaring emits removes those names from $attrs and changes fallthrough. The only current mount (FormView.vue:11) passes no listeners so it is safe today, but `created`/`cancel` are generic enough that a future mount adding @cancel would silently get a component emit instead of a native listener. Separately, the guard table says "don't register onBeforeRouteLeave" which reads like a runtime branch; it is a setup-time composition call and wrapping it in a conditional needs stating explicitly.
severity: significant
resolution: 'Emits renamed to inline-created / inline-cancelled so no future mount mistakes them for native listeners. Risk-table wording corrected from "byte-identical" to the accurate claim (the sole mount passes no listeners or attrs, so the fallthrough change is unobservable today). Guard table reworded: onBeforeRouteLeave is called at setup time inside if (!props.embedded), a conditional registration, and embedded is never reactive after mount.'
status: addressed
---

## Resolution

Emits renamed to `inline-created` / `inline-cancelled` — namespaced enough that
no future author reaches for them expecting a native listener, and they read
correctly at the one call site (the modal).

The risk-table wording is corrected from "byte-identical in behaviour" to the
accurate claim: the only existing mount passes no listeners and no attrs, so it
is unaffected; the fallthrough change is real but unobservable today.

The guard table is reworded to say `onBeforeRouteLeave` is **called at setup
time inside `if (!props.embedded)`** — a conditional registration, not a runtime
branch inside the guard. `embedded` is never reactive after mount, so the
conditional is sound.
