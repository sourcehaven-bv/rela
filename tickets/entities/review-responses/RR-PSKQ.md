---
id: RR-PSKQ
type: review-response
title: Multi-value comma encoding has no escape
finding: 'FilterBar uses val.split('','') for multi-select. URL encoding does NOT solve commas-in-values: %2C decodes to , before our code sees it. A tag named ''foo,bar'' becomes indistinguishable from two tags. Plan elevates this to a URL contract, making it harder to fix later. Use repeated query params (filter[tags][]=foo&filter[tags][]=bar) or document the constraint.'
severity: significant
resolution: 'Repeated query param form: filter[tags][in][]=a&filter[tags][in][]=b. Backend applyV1Filters extended to handle the [] suffix and join values. Fixes existing values[0] truncation. Documented in user guide.'
status: addressed
---
