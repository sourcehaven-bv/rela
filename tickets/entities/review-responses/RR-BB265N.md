---
id: RR-BB265N
type: review-response
title: No absent-id controls; hidden history costs one more read
finding: Hidden vs absent is not pinned for item, history, comments; hidden history reads MatchingFaces twice vs once.
severity: minor
resolution: Absent controls added (see RR-6C5477); the extra MatchingFaces read on hidden history is recorded in the corpus so a later fix shows as a diff.
status: addressed
---
