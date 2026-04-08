---
id: RR-NDXPK
type: review-response
title: setupRenameTestEnvWS wrapper was the wrong fix for dogsled lint
finding: Adding a one-line wrapper just to dodge the dogsled rule was uglier than the original problem. A struct return is cleaner and scales to future fields.
severity: minor
resolution: Replaced setupRenameTestEnv's tuple return with a renameTestEnv struct. Tests do `env := setupRenameTestEnv(t)` and access `env.ws`, `env.repo`, etc. The dogsled wrapper is gone; the test that previously needed it now just does `setupRenameTestEnv(t).ws` inline.
status: addressed
---
