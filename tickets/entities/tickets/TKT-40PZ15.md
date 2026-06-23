---
id: TKT-40PZ15
type: ticket
title: 'Pre-attach Lua veto hook: expose file info (name/size/content-type) to a write hook that can reject'
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

Give users a Lua escape hatch to **reject an attachment upload** based on the
incoming file's metadata (filename, size, content-type) — e.g. enforce a
per-property MIME allowlist or a tighter size cap for semi-untrusted users,
beyond the product-wide default limit.

Split out of TKT-RXFD5B after the read-path code review (TKT-Q85275) established
this is **net-new mechanism**, not a small binding addition.

## Why it's its own ticket (findings)

- **rela has no write-veto hook today.** Write-time Lua runs only via the automation engine, which fires **after** the entity is already persisted (`entitymanager/manager.go`: `upsertEntity` precedes `Cascade.Process`), and is explicitly fire-and-forget — `autocascade/runner.go`: "one bad script does not abort the cascade." Script errors land in `UpdateResult.AutomationErrors`, which **no write path consumes as a hard failure**. So there is no existing hook to extend; a veto is new behavior.
- **File size/content-type never reach the entity write path.** `attachment.Service.Attach` calls `Store.AttachFile(reader)` then stamps a **path string** onto the property and calls `UpdateEntity`. The entity carries only the path string; size isn't computed and content-type isn't persisted (both are re-derived from the filename on read). So the automation Lua sees `entity.properties.<prop>` = path string, never the bytes/size/content-type.
- **The only place that holds filename + reader before commit** is inside `attachment.Service.Attach` (between opening the file and `AttachFile`/`UpdateEntity`). A pre-attach veto must be invoked **there**, with a new Lua surface (a file-info table) and a new abort path.

## Scope

- A new **pre-attach hook** invoked from `attachment.Service.Attach` (covers CLI) and the new web upload handler (TKT-RXFD5B), *before* bytes are written, that:
  - exposes a Lua file-info table: `{ filename, size, content_type, property, entity_id, entity_type }`
  - lets the script reject (abort the attach with a clear error → 422/4xx on the web path)
- Respect `internal/dataentry/CLAUDE.md` "tolerate temporarily invalid data": this is an **explicit opt-in gate**, not a tightening of general validation.
- Decide the binding surface consistent with `ReadDeps`/`WriteDeps` (`internal/lua/deps.go`) and the "Lua only at write time" rule.

## Acceptance

- A project can declare a Lua hook that inspects an incoming attachment and rejects it; the reject surfaces as a clear error on both `rela attach` and the web upload, and **no bytes are persisted** on reject.
- Allow-path is unaffected; no hook declared → no behavior change.

## Notes

- Depends on TKT-RXFD5B (the web upload handler is one of the two host sites).
- The product-wide default size limit (TKT-RXFD5B) already covers the common "too big" case; this hook is for richer per-project policy.

Parent: FEAT-870YCY.
