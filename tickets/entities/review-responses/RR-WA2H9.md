---
id: RR-WA2H9
type: review-response
title: schemaStore.getEntityDetailView misuses computed
finding: 'stores/schema.ts:70: `computed(() => (type) => entityViewConfigs.value.get(type)?.detail_view)` — the outer computed body doesn''t read entityViewConfigs.value, only the inner closure does, at call time. The computed wrapper signals ''memoized derivation'' but provides neither memoization nor reactivity. It should just be a plain function on the store. Pre-existing code; not introduced here. Defer.'
severity: nit
reason: Pre-existing code in stores/schema.ts not introduced by this ticket. Calling it works correctly because the closure reads reactively at call time. Refactoring schema-store internals is out of scope for a frontend feature ticket; should be done in a focused store cleanup pass that touches all 'computed-returning-function' instances at once.
status: wont-fix
---
