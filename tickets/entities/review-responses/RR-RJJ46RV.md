---
id: RR-RJJ46RV
type: review-response
title: The fallback could use another type's link_text
finding: Nested node cells passed no entity type, so the lookup by property name could pick the link_text of another type.
severity: significant
resolution: EntityDetail passes the node's entity type; when types disagree on link_text the widget shows the URL.
status: addressed
---
