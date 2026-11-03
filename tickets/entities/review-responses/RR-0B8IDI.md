---
id: RR-0B8IDI
type: review-response
title: entity_views has no columns
finding: Step 4 says 'views.*.sections.** and entity_views.** are already editable by wildcard; the load check refuses busy there'. EntityViewConfig (config.go:166-178) holds detail_view, duplicate and mentioned_by_panel only, with no sections or columns. Drop entity_views from the sentence.
severity: nit
resolution: ADDRESSED entity_views dropped from step 4
status: addressed
---
