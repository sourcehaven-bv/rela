---
id: RR-SILQH
type: review-response
title: Three-way slash key coordination is brittle
finding: 'Three independent document.addEventListener(''keydown'') listeners all special-case slash. Two defer by querying document.querySelector(''.entity-list .search-box''). The order they fire depends on mount order. Better: a central registry pattern like modalStack.ts.'
severity: significant
reason: The slash-key registry refactor (modalStack-style ownership) is a real improvement, but extracting it touches Sidebar, useKeyboardShortcuts, useListKeyboard, and SearchView for a behavior that already works correctly via the document.querySelector probe. Filed as a follow-up. The current solution is documented inline so the next reader knows the deferral is by design.
status: deferred
---
