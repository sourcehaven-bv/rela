---
id: RR-LJYZGC
type: review-response
title: The job-queue postgres conformance suite never ran in CI
finding: just test-postgres runs ./internal/store/pgstore/... and ./internal/jobs/..., but CI's Postgres Backend job ran only the pgstore package. The durable job tier was therefore unpinned in CI despite the local recipe covering it — exactly the gap that let a backend-specific bug ship once already (a NUL byte in the idempotency fingerprint that Go and the memory backend accept and PostgreSQL rejects outright). A reviewer reached this conclusion via wrong reasoning (claiming RELA_TEST_DATABASE_URL was unset in CI, which it is not); the gap was real for a different reason.
severity: significant
resolution: Added a Run job-queue conformance step to the Postgres Backend job, with the same no-skip guard the pgstore step uses so a skipped suite cannot pass as green. Verified the suite passes against a real local PostgreSQL before asserting it in CI (126s, exit 0), rather than adding an unverified gate.
status: addressed
---
