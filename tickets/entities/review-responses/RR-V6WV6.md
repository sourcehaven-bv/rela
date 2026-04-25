---
id: RR-V6WV6
type: review-response
title: E2E specs use Date.now() suffixes for generated IDs
finding: Date.now() can collide across parallel workers running within the same millisecond and is brittle for cleanup if tests interleave. Per CLAUDE.md auto-generate-identifiers guidance.
severity: minor
reason: Real concern but the same Date.now() pattern is used across the existing e2e suite (forms.spec.ts, kanban.spec.ts, etc.). Replacing them all with a uniqueId(prefix) helper or crypto.randomUUID is a project-wide consistency fix worth doing, but should land as a single sweep rather than scattered across feature PRs. Filing a follow-up; in the meantime, the existing tests have not flaked on collision because Playwright's worker count and the test fixture's per-worker server isolation make sub-ms collisions effectively impossible.
status: deferred
---
