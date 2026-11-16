---
id: RR-93R5TOB
type: review-response
title: xargs without -r gave a confusing error on no matches
finding: With no grep matches, xargs ran dirname with no arguments and go test then failed on the repo root.
severity: minor
resolution: 'The script uses xargs -r and fails with an explicit ::error:: when no concurrency tests are found.'
status: addressed
---
