---
id: RR-IS39JPA
type: review-response
title: A share condition that cannot be evaluated is silent
finding: localRows skipped a row whose share condition errored without any report, so a broken condition would quietly stop pushing rows.
severity: minor
resolution: The run raises one issue per type with the count of rows share could not judge.
status: addressed
---
