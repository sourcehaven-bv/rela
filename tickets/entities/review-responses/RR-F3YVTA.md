---
id: RR-F3YVTA
type: review-response
title: sqlite world-page EXPLAIN case asserts almost nothing
finding: sortOK true let a regression to a whole-type sort pass; only the index name was checked.
severity: minor
resolution: Each case now lists its temp b-trees exactly. The world page must show LAST 2 TERMS OF ORDER BY plus one ORDER BY; HighestID one DISTINCT; all other reads none. Added a world page after a cursor case.
status: addressed
---
