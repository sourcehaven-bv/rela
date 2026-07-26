---
id: IMPL-SCXHUL
type: implementation-checklist
title: 'Implementation: relation-history UI e2e'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] `postgresTest` Playwright fixture: spawns rela-server-postgres against a per-test isolated schema (psql CREATE/DROP + search_path DSN), gated on RELA_E2E_DATABASE_URL, skips otherwise
- [x] Typed api helpers: `setRelationMeta`, `waitForRelationVersions` (explicit fromType)
- [x] `relation-history.page.ts` page object + `relation-history.spec.ts`: timeline + prop diff + restore; affordance present-outgoing/absent-incoming
- [x] `sweepConfigFromEnv` — env-tunable version-sweep cadence (RELA_VERSION_SWEEP_INTERVAL/_IDLE/_MAX_STALENESS) so captures appear in seconds; unit-tested
- [x] `:data-version` selector attribute on the timeline item
- [x] CI E2E job: postgres:16 service + build rela-server-postgres + env

## Quality

- [x] e2e typecheck + lint (Page Object Pattern) clean
- [x] Full fsstore e2e suite green (231 passed, relation-history skipped without DB)
- [x] Postgres e2e passes under 2 parallel workers (repeat x3)
- [x] Go: pgstore + appbuild (postgres) build/vet; sweep + purge tests green
