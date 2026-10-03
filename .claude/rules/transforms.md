---
paths:
  - "internal/transform/**"
  - "internal/cmdexec/**"
  - "internal/attachment/**"
  - "internal/dataentry/export*.go"
---

# View export & transforms (`internal/transform`)

The `transforms:` map in the metamodel registers named `markdown → format`
external commands (see `docs/transforms.md`). A `transform.Renderer` produces
markdown; the engine runs it through a transform via `internal/cmdexec` (argv
array, no shell, temp-file `{in}`/`{out}`, timeout, output cap — the same
security-reviewed exec pattern `internal/attachment` uses). Rules for new code:

- **Export is downstream of an already-authorized view, never a new
  capability.** Entity/list export in `internal/dataentry` routes through the
  SAME ACL read path as the view (`visibleReader.getVisible` /
  `scopedSortedEntities`); a request may only choose a registered transform
  _name_, never a command/flag/path.
- **The list-table renderer lives in `internal/dataentry`, not
  `internal/transform`** — it needs the ACL neighbor-visibility gate
  (`visibleRelationIDs`) so hidden neighbor titles never leak into an export.
  `internal/transform` must NOT import `internal/dataentry`; the built-in
  single-entity renderer lives in `transform`, and `dataentry` supplies the list
  renderer as a `transform.Renderer`.
- **The per-type render override (`views.<type>.export_render`) renders through
  `documentService.RenderMarkdown`** — the same Lua document machinery, reached
  only AFTER the export has resolved the entity through the ACL read gate. Never
  call `script.ExecuteDocument` on a fresh unauthenticated surface, and keep the
  entity id path-validated (`isSafePathSegment`) before it reaches a render.
- **Export downloads are hardened** like attachment downloads (nosniff, sandbox
  CSP, `no-store`, sanitized `Content-Disposition`) — the produced bytes embed
  user content.
- **External commands are CONFINED in `internal/cmdexec`, and it fails closed.**
  Both export and attachment processing run third-party parsers over
  attacker-influenceable bytes, so the shared runner adds: a no-network,
  temp-dir-only sandbox (bubblewrap on Linux, `sandbox-exec` on macOS), rlimits
  (memory/PIDs/file size/CPU, Linux), process-group kill so a converter's helper
  cannot outlive the timeout, and a bounded pool capping concurrent runs. On a
  host with no mechanism, commands REFUSE to run — only command execution is
  blocked, never server startup. Do not add a "can I run?" predicate: call `Run`
  and handle its error; `Describe()` exists solely for the startup log.
- **The transform engine must be built ONCE and shared**, not per request — it
  owns the bounded pool, so a per-request engine gives every request its own
  pool and the concurrency cap bounds nothing.
