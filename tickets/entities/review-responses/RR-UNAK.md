---
id: RR-UNAK
type: review-response
title: Empty phase-2 query uses '*' wildcard, diverges from EntityPickerModal pattern
finding: 'useBacktickAutocomplete.ts line 323 substitutes `''*''` when the partial id query is below MIN_SEARCH_LEN. Tracing the backend path (internal/dataentry/api_v1.go handleV1Search → executeQuery → runFreeTextSearch → internal/search/index.go Search → buildBoostedWordQuery), the `*` is detected as a wildcard and emitted as `NewWildcardQuery(''*'')` across all boosted fields (id 5x, primary 3x, properties 2x, content 1x). This works — it returns every document — but: (a) it goes through the full bleve scoring pipeline for what is conceptually a ''list all of type T'' operation; (b) wildcard `*` queries can be slow on large indexes; (c) it diverges from EntityPickerModal.vue which uses MIN_QUERY_LEN and shows no results below it, so users experience two different idle-state behaviors across the two pickers. Better: either reuse the list-entities endpoint (`/api/v1/<plural>`) for the zero-query case, or follow the EntityPickerModal pattern and show a ''type to search'' hint when below MIN_SEARCH_LEN. The current choice is the most expensive option.'
severity: significant
resolution: 'runSearch now branches on query length: empty/short calls listEntities(type) which honors the type''s default sort; longer queries go through searchEntities (Bleve). No more ''*'' wildcard hack. Two unit tests pin both pathways: empty -> listEntities, non-empty -> searchEntities with correct type arg.'
status: addressed
---
