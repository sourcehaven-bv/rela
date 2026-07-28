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
retroactive): 1121 artifacts deleted, 0 failures, org storage verified down to
214 MB.
