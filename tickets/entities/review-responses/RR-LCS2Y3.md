---
id: RR-LCS2Y3
type: review-response
title: JSON-null sort keys ordered differently on the pushed path vs the Go path
finding: 'Both Go comparators keyed on map-key presence; a property present with a nil value compared as the text "<nil>" and landed between dates and absent rows, while PostgreSQL''s ->> yields NULL and sorts it last. Reachable: pgstore.unmarshalProps preserves JSON null as a present key, and YAML `due:` produces it. Both fixtures seeded only absent keys.'
severity: critical
resolution: graphquerynaive.sortValue and applyV1Sorting treat a nil value as absent (largest), matching SQL; RunGraphPagingTests seeds a JSON-null row (T-6) with pinned asc/desc orders and count on all four backends; the listpushdown differential fixture seeds two null rows.
status: addressed
---
