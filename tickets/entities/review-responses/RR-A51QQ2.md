---
id: RR-A51QQ2
type: review-response
title: Deep-link title→option reverse resolution + option values must be titles
finding: On page load with filter[verantwoordelijk_voor]=Jeroen Vloothuis in the URL, initializeFilters (FilterBar.vue:95) seeds localFilters[relation] with a TITLE string, while candidates are keyed by id. The selector must show the correct option as selected by matching the incoming committed title against entityDisplayTitle(candidate). This can be ambiguous (two entities, same title) — pick first-match, but state it. For the plain <select> case the <option> VALUES must be titles (not ids) so v-model=localFilters[relation] binds directly, matching today's property-select pattern (FilterBar.vue:229). For typeahead, explicit title→selected-candidate resolution is needed since there is no <option> set.
severity: significant
resolution: 'Plan specifies: <select> option values are bare titles (v-model binds the title directly, trivial round-trip). Typeahead resolves the committed title against entityDisplayTitle(candidate) to mark the selected option; first-match wins on duplicate titles (documented). Consistent with the title-as-value contract from RR-X4QWBF.'
status: addressed
---
