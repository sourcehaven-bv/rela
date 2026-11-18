---
id: RR-RM2WFQU
type: review-response
title: A transient refetch failure cleared the chart
finding: The catch block cleared the series on every error, against the keep-content rule in frontend/CLAUDE.md.
severity: significant
resolution: Series are cleared only when shouldDropHeldContent is true; other failures keep the lines and show the failed note only with nothing on screen. Added a refetch test with a non-denial error.
status: addressed
---
