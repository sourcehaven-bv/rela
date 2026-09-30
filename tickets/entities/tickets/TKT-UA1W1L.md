---
id: TKT-UA1W1L
type: ticket
title: Remove the sync feature
kind: chore
priority: medium
effort: m
status: done
description: 'Stage 3 PR 10 of TKT-7IZHP0 (ruling D9): delete the unused sync feature (code, CLI, HTTP routes, docs, tests).'
---

## Description

Stage 3 PR 10 of TKT-7IZHP0. Ruling D9: sync is unused, so remove it entirely.
It can return in a follow-up.

Remove `internal/sync`, `rela sync`, the `/api/sync/` routes and handlers,
`pgstore.ManifestSince`, `entitymanager.ApplyEntity`/`ApplyRelation`,
`principal.ToolSync`, their tests, guard allowlist entries, arch-lint rules and
docs.

Keep what other features share:

- the pgstore `deletions` table and seq indexes (change-feed catch-up);
- the non-browser CSRF exemption on `/api/v1` data routes (documented curl use);
- `internal/canonical` (versioning dedup, relation ETag);
- the single-relation GET (public v1 API);
- `RecreateEntity`, now create-only (history restore).

No drop migration: nothing sync-only is left in the database.

## Acceptance

- No sync package, command, route or doc remains.
- All tests, arch-lint, lint, comment-lint, plimsoll and coverage pass.
