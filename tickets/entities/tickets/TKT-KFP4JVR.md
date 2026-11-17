---
id: TKT-KFP4JVR
type: ticket
title: Run Frontend CI in the Playwright image and retry apt steps
kind: chore
priority: high
effort: xs
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

Since 2026-11-11 the `E2E` and `Frontend` CI jobs fail at `npx playwright
install --with-deps chromium`. The `--with-deps` part runs apt-get on the
runner, and the Ubuntu mirror fails with "Some index files failed to download"
(exit 100). The `Demos` job fails on the same mirror.

## Change

- Frontend runs in Playwright's official image, pinned by version and digest
to the lockfile: `mcr.microsoft.com/playwright:v1.64.0-noble`, as uid 1001. The
browser install step is gone, so the job runs no apt.
- E2E stays on the runner. Its bubblewrap install and
`npx playwright install --with-deps chromium` are retried up to three times.
- Every apt-get step in ci.yml retries `apt-get update` up to three times and
passes `-o Acquire::Retries=5`.
