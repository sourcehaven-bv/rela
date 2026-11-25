---
id: BUG-T4W8P2X
type: bug
title: Release workflow tests time out at the default 10 minutes
description: release.yml ran go test -race without a timeout, so internal/dataentry hit the 10 minute default and failed the v26.11.6 and v26.11.7 releases.
priority: medium
effort: xs
why1: The Test job failed with a package timeout after 600 s.
why2: internal/dataentry takes about 10 minutes under -race.
why3: release.yml used go test's default timeout.
why4: ci.yml was given -timeout 20m (TKT-A6A97WV) but release.yml was not.
why5: Two workflows run the same suite with separate flags.
prevention: release.yml uses the same -timeout 20m as ci.yml.
completed: "2026-11-24"
status: done
---

The Release workflow failed on the Test job with "test timed out after 10m0s".
Fixed by passing -timeout 20m, as ci.yml does.
The first PR (#2591) carried only the ticket files; this one carries the workflow change. v26.11.8 failed the same way.
