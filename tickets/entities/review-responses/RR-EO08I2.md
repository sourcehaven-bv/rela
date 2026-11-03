---
id: RR-EO08I2
type: review-response
title: SPA ignored truncated and kept an unreachable note
finding: MentionedByPage had no truncated field, and the 'Nothing you can see yet' branch could no longer render once a cursor implied items.
severity: minor
resolution: The page type carries truncated; the panel shows 'Some entities that mention this one could not be listed.' and the dead branch is gone. Vitest covers it.
status: addressed
---
