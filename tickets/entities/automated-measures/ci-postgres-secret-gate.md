---
id: ci-postgres-secret-gate
type: automated-measure
title: 'Measure: Postgres CI uses trust auth (no secret) so it runs on Dependabot/fork PRs'
description: 'Control for BUG-RXIUCT. The Postgres Backend CI job runs an ephemeral, runner-local postgres:16 with trust auth (no password, no secret), so it executes on every PR including Dependabot and fork PRs (where repository secrets are unavailable). The previous secret-sourced password made the container fail to start on those runs and cascade-blocked Build. Verified: actionlint exit 0 and a live same-repo run with Postgres Backend green under trust auth.'
kind: ci
location: '.github/workflows/ci.yml (postgres job: POSTGRES_HOST_AUTH_METHOD=trust; DSN without password)'
status: active
---
