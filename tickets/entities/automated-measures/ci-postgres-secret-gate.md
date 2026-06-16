---
id: ci-postgres-secret-gate
type: automated-measure
title: 'Measure: Postgres CI job gated on secret presence; Build tolerates skipped postgres'
description: Control for BUG-RXIUCT. The `pg-secret-gate` job reports whether `CI_POSTGRES_PASSWORD` is available; the `Postgres Backend` job runs only when it is, otherwise skipping cleanly (Dependabot / fork PRs). The `Build` job's condition was changed to require all other jobs to succeed and postgres to be success-or-skipped, so a legitimate skip does not cascade-skip Build while a real postgres failure still blocks it. Verified with actionlint (exit 0). This is the reusable gate-then-skip pattern for any future secret-gated service-container job.
kind: ci
location: '.github/workflows/ci.yml (pg-secret-gate job; postgres job `if: needs.pg-secret-gate.outputs.enabled == ''true''`; Build job `if:` tolerating postgres success-or-skipped)'
status: active
---
