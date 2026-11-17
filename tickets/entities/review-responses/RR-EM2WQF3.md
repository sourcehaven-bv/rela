---
id: RR-EM2WQF3
type: review-response
title: Paths could not name a field with spaces
finding: CSV headers such as Control ID could not be mapped; filter values could not hold ].
severity: significant
resolution: Added the ['name'] step; a quoted filter value may hold ]. Documented in the guide.
status: addressed
---
