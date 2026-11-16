---
id: RR-YELRCA4
type: review-response
title: CI wiring details missing
finding: rela-tickets job (ci.yml:1654) builds bin/rela but has no setup-just; without --require-tests deleting tickets/tests passes; dependabot PRs skip the job.
severity: minor
resolution: Plan calls ../bin/rela flow validate and ../bin/rela test --require-tests directly in the rela-tickets job (no just recipe); notes that push and merge_group runs still execute it.
status: addressed
---
