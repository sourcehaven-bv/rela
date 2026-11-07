---
id: RR-2LW53NZ
type: review-response
title: Unbounded OSV paging loop
finding: A repeated next_page_token would loop until the job timeout
severity: minor
resolution: Capped at 20 pages; fails if still paging
status: addressed
---
