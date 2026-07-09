---
id: RR-OAQMDM
type: review-response
title: Badge ellipsis max-width applies to all badges (unlabeled regression)
finding: 'Badge.vue put `max-width: 24ch` + nowrap + ellipsis on the base `.badge` rule, so a pre-existing raw enum value >24 chars that rendered fully before now truncates — a rendering change for badges unrelated to the feature. Scope the truncation to `.badge--labeled` (labels are the overflow risk this ticket introduced), leaving unlabeled badges unchanged.'
severity: significant
resolution: 'Moved max-width: 24ch + overflow ellipsis + nowrap from the base .badge rule to .badge--labeled, so unlabeled raw-value badges render exactly as before; only author-supplied labels (the new overflow risk) are truncated.'
status: addressed
---
