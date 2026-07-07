---
id: RR-NH8B6D
type: review-response
title: Typeahead search-query state must stay separate from committed value
finding: 'A typeahead has TWO states: (a) in-progress search query (what the user types to filter candidates) and (b) committed value (title of the clicked option). Excluding the relation widget from textWidgetKeys (FilterBar.vue:83) is correct for the committed value (changes only on click, like select). BUT if the extraction naively two-way-binds the search input to the committed value, an external props.filters update (back/forward nav, SSE, other tab) reassigns localFilters (:137) and clobbers the user''s in-progress typing, and a stale half-typed string can leak into buildState() on next emit. Clicking away without selecting must also not commit the search string.'
severity: significant
resolution: 'Plan specifies: searchQuery is component-LOCAL to EntityTargetSelect, strictly separate from the committed value. Committed value changes ONLY on option-select (emit up to localFilters[relation]). External props.filters change updates the SELECTED value (what shows as current selection), never the search box. Click-away discards search string without committing. Selector emits via handleFilterChange (immediate), not handleTextInput (debounced).'
status: addressed
---
