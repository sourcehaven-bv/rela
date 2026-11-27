---
id: RR-D5LBK3S
type: review-response
title: "Go test reads NAMED_COLORS by its export keyword"
finding: "knip removed `export` from NAMED_COLORS in styleColors.ts; TestColorNamesMatchSPA matches `export const NAMED_COLORS` and fails."
severity: critical
resolution: "The test regex accepts `(?:export )?const NAMED_COLORS`."
status: addressed
---

knip removed `export` from NAMED_COLORS in styleColors.ts; TestColorNamesMatchSPA matches `export const NAMED_COLORS` and fails.
