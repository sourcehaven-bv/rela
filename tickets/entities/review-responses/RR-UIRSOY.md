---
id: RR-UIRSOY
type: review-response
title: Postgres variant of CloseWaitsForRunningHandlers passed without the fix
finding: The handler slept 250ms after the cancel, under neoq's 1s cancel grace, so the postgres Shutdown already waited for it.
severity: minor
resolution: 'The handler now sleeps 1.5s after the cancel. Verified: the postgres variant fails 3 of 3 runs without the fix and passes with it.'
status: addressed
---
