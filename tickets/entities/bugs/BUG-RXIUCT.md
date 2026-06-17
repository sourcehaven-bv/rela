---
id: BUG-RXIUCT
type: bug
title: 'CI: Postgres Backend job fails on Dependabot/fork PRs (empty CI_POSTGRES_PASSWORD), blocking Build'
description: |-
    The `Postgres Backend` job in `.github/workflows/ci.yml` sourced the postgres service container's `POSTGRES_PASSWORD` solely from `secrets.CI_POSTGRES_PASSWORD` (no fallback, per the TKT-M8400 security audit). GitHub does not expose repository secrets to workflows triggered by Dependabot or by PRs from forks, so there the secret resolves to empty and the `postgres:16` service container refuses to initialise (`Error: Database is uninitialized and superuser password is not specified`). The job fails in ~10s on every such PR. Because the `Build` job has `needs: [..., postgres, ...]`, the postgres failure also blocks `Build` (and the downstream `demos`/`docs`). Observed on Dependabot PR #979 (esbuild bump), where every frontend-relevant check passed but CI was red on the unrelated postgres infra failure.

    **Fix (final):** the CI database is ephemeral, holds no real data, and is reachable only from the job's own runner — there is no credential to protect. So the secret-sourced password was removed entirely in favour of trust auth: the container sets `POSTGRES_HOST_AUTH_METHOD: trust` (no password) and the test DSN drops the password component (`postgres://rela@127.0.0.1:5432/rela_test?sslmode=disable`). This lets the Postgres Backend job run everywhere, including Dependabot and fork PRs, with no secret dependency. (An earlier iteration gated the job to skip when the secret was absent; that was replaced by trust auth once it was confirmed the password was never a real credential, so coverage on Dependabot PRs is preserved rather than skipped.) Supersedes the TKT-M8400 'secret-only' decision for this throwaway CI database.
priority: medium
effort: s
why1: The postgres service container failed to start because `POSTGRES_PASSWORD` was empty.
why2: '`POSTGRES_PASSWORD` is sourced solely from `secrets.CI_POSTGRES_PASSWORD`, which GitHub redacts to empty for Dependabot- and fork-triggered workflow runs.'
why3: The TKT-M8400 security audit chose secret-only with no fallback so the job would 'fail loudly' if the secret were removed — but that rule also makes it fail on every run where secrets are legitimately unavailable, not just on removal.
why4: There was no branch in the workflow distinguishing 'secret intentionally absent (Dependabot/fork)' from 'secret misconfigured', so the only outcome was a hard failure.
why5: Service-container jobs that depend on a secret have no built-in skip-when-unavailable affordance; the workflow needed an explicit gate (the repo already uses one for the rela-tickets job) to express 'run only when the prerequisite secret exists'.
prevention: 'Postgres CI uses trust auth (no secret), so the job runs on every PR including Dependabot/fork — there is no secret-availability failure mode left. Rule of thumb: do not gate an ephemeral, runner-local CI service container behind a repository secret; secrets are unavailable on Dependabot/fork runs and a throwaway DB has no credential worth protecting. Verified with actionlint (exit 0) and by a live same-repo CI run where Postgres Backend passed under trust auth.'
status: done
---
