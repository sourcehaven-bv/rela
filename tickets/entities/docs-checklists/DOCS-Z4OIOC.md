---
id: DOCS-Z4OIOC
type: docs-checklist
title: 'Docs: Replace the shape-hash migration chain with a committed applied-list (TKT-XCJ0Y2)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have doc comments
- [x] Non-obvious decisions explained with rationale
- [x] Package docs updated where behaviour changed

`internal/datamigration/doc.go` was rewritten around the two questions the
package now answers separately (which migrations have run, by name; what shape
the data conforms to, by projection), including why both projections stay
embedded in each file and why the applied list being the only double-apply guard
makes step idempotency load-bearing.

Marker-era wording was corrected in `gate.go`, `run.go`, `generate.go` and
`lock.go` — the last including a note that the lock deliberately stayed in
`.rela/` when the record moved into the project tree, because a lock is machine
state and must not be committed.

## Project Documentation

- [x] `docs-project/entities/` updated (generated docs rebuilt with `just docs`)
- [x] CLAUDE.md updated
- [x] Project-layout listing updated

- **`GUIDE-data-migration.md`** — rewritten: the projection vs. applied-list
split, the per-backend record table, timestamp naming and why sequence numbers
collide, a **Data-only migrations** section, the classify-vs-persist split, the
un-baselined refusal (every command, not just `status`), the upgrade-from-legacy
path, name validation, and the idempotency-is-now-the-only-guard warning.
- **`GUIDE-cli-reference.md`** — `migrate baseline` added to the command list and
flag table, with a note that it makes no attempt to verify the operator's claim.
- **`GUIDE-postgres-backend.md`** — the per-tenant paragraph now names
`migration_state` rather than `state_kv` rows.
- **CLAUDE.md** — the data-migration bullet rewritten; `migrations/applied.json`
added to the project-files listing.

Generated `docs/*.md` rebuilt from the entities; `just ci`'s docs-freshness gate
passes.

## External Documentation

- [x] ~~README changes~~ (N/A: no top-level feature surface changed)
- [x] ~~API/schema reference~~ (N/A: no API or metamodel surface changed)

## Verification

- [x] Docs match the shipped behaviour

The end-to-end walkthrough in IMPL-J5RMKW follows the documented workflow, and
the un-baselined refusal was re-verified after the code-review fixes changed it
from "status refuses" to "every command refuses".
