---
id: RR-ZDA1VE
type: review-response
title: List-end marker leaks into markdown
finding: Adjacent lists produced <!--THE END--> in the stored markdown.
severity: minor
resolution: The marker is removed; adjacent lists merge and the fixed point settles.
status: addressed
---
