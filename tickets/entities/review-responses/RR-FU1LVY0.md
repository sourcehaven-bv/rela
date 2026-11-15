---
id: RR-FU1LVY0
type: review-response
title: pick_one ignores its limit on free-text queries
finding: pick_one passed only firstN, which applies to the branch without free text, so a free-text pick_one query kept the 1000-hit relevance cap.
severity: minor
resolution: pick_one passes limit as well, so freeTextLimit lowers the cap where that is safe.
status: addressed
---
