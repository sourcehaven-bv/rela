---
id: BUG-RXIUCT
type: bug
title: 'CI: Postgres Backend job fails on Dependabot/fork PRs (empty CI_POSTGRES_PASSWORD), blocking Build'
description: |-
    The `Postgres Backend` job in `.github/workflows/ci.yml` sets the postgres service container's `POSTGRES_PASSWORD` solely from `secrets.CI_POSTGRES_PASSWORD` (no fallback, per the TKT-M8400 security audit). GitHub does not expose repository secrets to workflows triggered by Dependabot or by PRs from forks, so there the secret resolves to empty and the `postgres:16` service container refuses to initialise (`Error: Database is uninitialized and superuser password is not specified`). The job fails in ~10s on every such PR. Because the `Build` job has `needs: [..., postgres, ...]`, the postgres failure also blocks `Build` (and the downstream `demos`/`docs`). Observed on Dependabot PR #979 (esbuild bump), where every frontend-relevant check passed but CI was red on the unrelated postgres infra failure.

    **Fix:** add a secret-less `pg-secret-gate` guard job that reports whether `CI_POSTGRES_PASSWORD` is present; gate the `postgres` job on it so it skips cleanly (rather than failing) when the secret is unavailable. Update the `Build` job's condition to tolerate a *skipped* postgres (`always() && <required jobs success> && (postgres success || skipped)`) so a legitimate skip no longer cascade-skips Build, while a genuine postgres failure still blocks it. Preserves the security property (secret stays authoritative, never hardcoded) for normal pushes and same-repo PRs.
priority: medium
effort: s
why1: The postgres service container failed to start because `POSTGRES_PASSWORD` was empty.
why2: '`POSTGRES_PASSWORD` is sourced solely from `secrets.CI_POSTGRES_PASSWORD`, which GitHub redacts to empty for Dependabot- and fork-triggered workflow runs.'
why3: The TKT-M8400 security audit chose secret-only with no fallback so the job would 'fail loudly' if the secret were removed — but that rule also makes it fail on every run where secrets are legitimately unavailable, not just on removal.
why4: There was no branch in the workflow distinguishing 'secret intentionally absent (Dependabot/fork)' from 'secret misconfigured', so the only outcome was a hard failure.
why5: Service-container jobs that depend on a secret have no built-in skip-when-unavailable affordance; the workflow needed an explicit gate (the repo already uses one for the rela-tickets job) to express 'run only when the prerequisite secret exists'.
prevention: Added the `pg-secret-gate` guard job and a `(success || skipped)` tolerance on `Build`. The pattern mirrors the existing `rela-tickets` Dependabot/chore short-circuit. Verified the workflow with actionlint (exit 0). Future secret-gated service jobs should follow the same gate-then-skip pattern rather than relying on a hard failure.
status: done
---
