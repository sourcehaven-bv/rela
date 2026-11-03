---
id: RR-SPJE133
type: review-response
title: A DTSTART with a time of day returned the after date itself
finding: NextRrule truncated to the day and asked for the first occurrence after midnight, so DTSTART 09:00 on the after date gave that same date.
severity: significant
resolution: NextRrule asks for the first occurrence on or after the next midnight; pinned by the DTSTART-with-time case in TestNextRrule.
reason: NextRrule now asks for the first occurrence on or after the next midnight. Pinned in TestNextRrule.
status: addressed
---
