---
id: RR-WFF7Z
type: review-response
title: Metamodel still loads from disk — postgres build still needs a project dir
finding: The plan implied a postgres deployment is DB-only, but appbuild.New (line 411) and mcp_wiring.go (line 62) ALWAYS load the metamodel via metamodel.NewFSLoader(fs, paths.MetamodelPath), and project.Discover walks the filesystem for metamodel.yaml + .rela cache dir + templates. The metamodel is NOT in the database. So even -tags postgres requires a filesystem project directory containing metamodel.yaml (and templates/ for defaults). This wasn't stated as a requirement/constraint and changes the deployment story and the integration-test setup (the test must provision both a Postgres DSN AND a project dir with metamodel.yaml).
severity: significant
status: open
---

## Resolution (plan update)

- Document explicitly: **the postgres build still needs a filesystem project directory** providing `metamodel.yaml` (+ optional `templates/`, `.rela/` cache). PostgreSQL backs entities/relations/attachments/search only — schema/config stays on disk.
- `rela-server`/`rela`/MCP in the postgres build still take `--project` (or discover from cwd) for the metamodel; `--database-url` is additive.
- Integration & conformance tests must provision BOTH: a Postgres DSN and a minimal project dir (or a `metamodel.Metamodel` constructed in-memory for the store-level conformance tests, which already pass `meta` directly to the store).
- Out of scope (note for later): moving metamodel into the DB. Not in this ticket.
