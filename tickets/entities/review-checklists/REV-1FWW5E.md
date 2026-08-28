---
id: REV-1FWW5E
type: review-checklist
title: 'Review: drop unused rela-linux binary artifact (TKT-EWNORS)'
status: done
---

## Automated Checks

- [x] Full CI green on the PR (all jobs except this ticket gate passed first run)
- [x] `Build` job still passes — the compile gate is intact without the upload
- [x] YAML parses; `build` job retains checkout / setup-go / Build steps

## Verification

- [x] Confirmed no `download-artifact` step in any of the 7 workflows references `rela-linux`
- [x] Confirmed `release.yml` is independent: GoReleaser builds the CLI from a fresh
checkout and the desktop matrix builds its own binaries — neither consumes CI
artifacts
- [x] Confirmed no remaining `rela-linux` reference anywhere in the repo
- [x] Checked branch protection for required status checks before touching the job
(neither `main` nor `develop` is protected, so no required check was removed)

## Code Review

- [x] ~~cranky-code-reviewer agent run on the diff~~ (N/A: deletion of one workflow
step, no logic to review)
- [x] Change is a pure removal plus an explanatory comment

**Summary:** The `build` job uploaded `bin/rela` as `rela-linux` on every
push/PR and nothing ever downloaded it. At 90-day retention this had grown to
24.6 GB across ~1100 artifacts — effectively the whole org's Actions storage
against a 2 GB quota. The upload step is removed; the job stays as a compile
gate.

The 24.6 GB backlog was purged separately (retention changes are not
retroactive): 1121 artifacts deleted, 0 failures. Live (unexpired) storage
verified down to ~214 MB, all of it `coverage-report` plus a few small
sarif/fuzz artifacts.

Two clarifications on that figure, prompted by review (IB-review #1244):

- **Expired rows still list.** The artifacts API returns already-expired
  artifacts with `"expired": true`; ~1143 such `rela-linux` rows (~17.4 GB
  nominal) remain enumerable. They are a disjoint set from the 1121 deleted
  (zero id overlap) — GitHub had already expired them at their 90-day mark, so
  they were never mine to delete and hold no billable storage. Any count of
  remaining `rela-linux` must filter on `expired == false`, or it double-counts
  bytes that no longer exist. The 1121 deletions were re-verified: all return
  404 individually and none appear in a fresh full listing.
- **Two stragglers, since removed.** Two runs on `develop` (`b98deaa6`,
  `4333c858`, both pushed 16:03–16:04 UTC) started before this PR merged and so
  still ran the old workflow, uploading 45 MB after the purge. Both were
  deleted. `develop` no longer contains the upload step, so no further
  `rela-linux` artifacts can be produced.
