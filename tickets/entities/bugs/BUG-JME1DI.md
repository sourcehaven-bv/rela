---
id: BUG-JME1DI
type: bug
title: Conflict endpoints allow path traversal and bypass ACL/audit
description: GET /api/v1/_conflicts/{path} and POST /api/v1/_conflicts/resolve joined the caller-supplied path onto the project root without containment, allowing reads and writes outside the project. The resolve write also bypassed ACL re-authorization and the audit log entirely (direct os.WriteFile).
priority: high
why1: The conflict handlers joined user-supplied paths with filepath.Join(root, path) and conflict.ResolveAndWrite wrote files directly, with no containment check, no ACL gate, and no audit record.
why2: The conflict endpoints predate the containedProjectPath helper and the ACL/affordances arc; they were never swept when those gates were introduced for the other write handlers.
why3: There is no structural enforcement that every dataentry mutation surface routes through the ACL gate — the affordances contract test covers entity CRUD handlers but not file-level endpoints like conflict resolve.
why4: Conflict resolution cannot route through entitymanager (the store cannot parse marker-laden files), so it sat outside the manager-centric write-path rules that guarantee ACL+audit.
why5: Write surfaces that bypass the Manager have no checklist or lint that forces them to re-implement the gate explicitly.
prevention: 'Backend review sweep ticketed: enumerate every mutation surface and require each to be ACL-gated and audited or documented as an exemption (review doc theme A). translateRelationWrite now lives in affordances.go under the lint-test single-construction-site invariant, and regression tests pin traversal rejection, the 403 deny body, and the audit rows.'
status: done
---

## Bug

Found in a full backend code review (2026-06-09). Two independent review passes
converged on the `_conflicts` endpoints:

- **Path traversal, read and write.** `GET /api/v1/_conflicts/../../secret` read any file the process could read; `POST /api/v1/_conflicts/resolve` with `"path": "../..."` wrote attacker-controlled bytes outside the project root. `requireSameOrigin` blocked cross-origin browsers, but same-origin XSS or any non-browser loopback client reached this.
- **Write-path bypass.** `conflict.ResolveAndWrite` did a raw `os.WriteFile` — no ACL re-authorization (violates the dataentry rule "the write endpoint must re-authorize"), no audit record.

## Fix (PR #947)

- Both endpoints contain the caller-supplied path via `containedProjectPath` (403 outside root, 404 missing).
- Resolve authorizes against the resolved write target before writing: entity files via `translateVerb("update", ...)`, relation files via the new `translateRelationWrite` (mirrors `entitymanager.UpdateRelation` semantics).
- `denied-write` audit row on deny; `update-entity` / `update-relation` row on success.
- Handler serializes under `writeMu` like every other mutation handler.
- `conflict.ResolveAndWrite` split into `Resolve` → `ValidateResolved` → `WriteResolved` so the gate fits between resolve and write; the write stays file-level because the store cannot parse a marker-laden file, and the store's file watcher propagates the change to index/SSE.

Seven regression tests in `internal/dataentry/conflicts_api_test.go`.
