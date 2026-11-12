---
id: RR-UUOE7BZ
type: review-response
title: HTTP thread zero returned the unfiltered page
finding: thread=0 passed parsing and meant no filter.
severity: minor
resolution: HTTP refuses a thread below 1 when the parameter is present, like MCP and CLI.
status: addressed
---
