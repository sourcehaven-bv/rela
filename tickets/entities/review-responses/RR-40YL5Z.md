---
id: RR-40YL5Z
type: review-response
title: Traverse use counted as showing backlinks
finding: validateMentionsUsed accepted a backlinks collection read only by another traverse step, so the backlinks vanished while the panel was hidden.
severity: minor
resolution: Only section sources and children count. New validation case "only another step reads it".
status: addressed
---
