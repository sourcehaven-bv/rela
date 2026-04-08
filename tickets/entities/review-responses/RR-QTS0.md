---
id: RR-QTS0
type: review-response
title: FilterBar clobbers in-progress text input on external nav
finding: 'User scenario: types ''foo'' into a text filter (debounce pending), then an external nav (back button or another emitter) updates filters.value. The watch(() => props.filters) in FilterBar runs initializeFilters which overwrites localFilters — dropping ''foo''. The pending debounce timer then fires with the new (post-overwrite) localFilters. User input vanishes silently. Fix: in the props.filters watcher, if textDebounceTimer is active, either flush-before-replace or merge so text-input fields preserve typed-in values.'
severity: significant
resolution: FilterBar.vue props.filters watcher now preserves text-widget values when textDebounceTimer is non-null (user is mid-type). Added textWidgetKeys computed so only text widgets are shielded — selects still reset to the incoming state. The pending debounce timer eventually fires with the preserved user input, so external nav doesn't silently discard keystrokes. textDebounceTimer declaration was hoisted above the watcher so the guard can reference it.
status: addressed
---
