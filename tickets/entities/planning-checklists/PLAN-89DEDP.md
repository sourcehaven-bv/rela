---
id: PLAN-89DEDP
type: planning-checklist
title: 'Planning: Attachment web write path: upload endpoint + drag-drop widget + default size limit + Lua file info'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements understood
- [x] Scope defined (below)
- [x] Acceptance criteria with test scenarios

**Scope:** IN: web upload (PUT) + delete (DELETE) on the `_attachments` route;
up-front `update`-ACL re-authorization; product-wide default size limit (ingress
`http.MaxBytesReader` 413 + fsstore/memstore backstop guards + configurable
default); drag-drop/file-picker on FileWidget gated on `_actions.update`. OUT:
Lua file-info veto (split to TKT-40PZ15), multi-file `max` (TKT-WLLRO7).

**Acceptance Criteria:**
1. Upload/replace/view/delete end-to-end on fsstore + pgstore.
2. Over-limit rejected on ALL backends — handler 413 + per-store guard rejects (unit-tested fs/mem/pg).
3. No `update` access → upload/delete denied BEFORE bytes written (acl test).
4. Orphan window closed for web path: deny/validation-fail leaves no orphan bytes.

## Research

- [x] Codebase patterns checked (two research passes)

**Prior art (file:line):**
- Method-dispatch the `_attachments` route case (api_v1.go ~354) on `r.Method`: GET→existing read handler, PUT→upload, DELETE→delete.
- Write-ACL: `translateVerb("update", type, id)` (affordances.go:34) + `a.acl.AuthorizeWrite`; deny→`writeForbiddenIfACLDenied` (handlers_api.go:354) + audit `OpDeniedWrite`. Mirror `authorizeConflictResolve` (api_v1.go:3080). MUST NOT construct `acl.WriteRequest{Op:` directly (lint_test.go grep; only affordances.go allowed).
- Multipart: `http.MaxBytesReader` + `ParseMultipartForm` + `FormFile` + `*http.MaxBytesError`→413 (handlers_theme.go:71-110). But emit 413 via `writeV1Error` (problem+json), not the theme's ad-hoc shape.
- Write/persist: `a.store.AttachFile` then `e.SetString(prop, key)` + `a.entityManager.UpdateEntity` under `a.writeMu.Lock()` (mirrors attachment.Service.Attach attachment.go:107-119 and handleV1UpdateEntity api_v1.go:1059).
- Size guard: pgstore `maxAttachmentBytes=64<<20` (pgstore/attachment.go:22); fsstore (writeAttachment 67-86) + memstore (669) unbounded → add backstop. Config home: dataentryconfig.Config (no limit field today).
- Store: `AttachmentManager` (store.go:218); `App.store` + `App.entityManager` (app.go:109).
- router_walk_test + lint_test enforcement understood.

**Research Doc:** N/A

## Approach

- [x] Chosen, builds on patterns, alternatives considered

**Technical Approach:** *Route.* Replace the straight `_attachments`→GET
dispatch with a method switch → `handleV1AttachmentRoute` that fans to
get/put/delete. *Upload (`handleV1PutAttachment`).* writeMu.Lock →
gateReadOrNotFound → load entity + isFileProperty (404 else) →
**AuthorizeWrite(translateVerb("update",…)) up front** (deny→403 before bytes,
audit) → `http.MaxBytesReader(maxAttachmentUploadBytes)` + ParseMultipartForm +
FormFile("file") → MaxBytesError→413 via writeV1Error → `a.store.AttachFile` →
`e.SetString(prop, key)` + `a.entityManager.UpdateEntity` (validation→422 via
writeForbiddenIfACLDenied then writeV1Error) → 200 with the V1Entity (so
`_attachments` reflects the new file). *Delete (`handleV1DeleteAttachment`).*
same gate+authorize → clear property via UpdateEntity FIRST → then
`a.store.DeleteAttachment` (idempotent) → 204. Property-first so a
validation/deny failure leaves bytes, not a dangling ref. *Size limit.* New
const `DefaultMaxAttachmentBytes` (start 64 MiB) + optional `dataentryconfig`
override; handler caps at ingress. Add `maxAttachmentBytes` guard
(io.LimitReader+length check, mirroring pgstore) to fsstore.AttachFile and
memstore.AttachFile so the store rejects oversize regardless of caller — store
returns a sentinel/error the handler maps. *Frontend.* Extend FileWidget: in
edit mode render a file `<input>` + drag-drop dropzone (replace the read-only
note), POST multipart to the href, show progress, replace/remove (DELETE). Gate
on `_actions.update`. New `api/attachments.ts` client. Inline 413/422 errors.

**Files:** handlers_attachment.go (+put/delete/dispatch), api_v1.go (route
line), dataentryconfig/config.go (limit field), store/fsstore/attachment.go +
store/memstore/memstore.go (guards) + store/storetest (conformance for the cap),
handlers_attachment_test.go (+upload/delete/acl/size), router_walk_test.go
(PUT/DELETE probes); frontend FileWidget.vue, api/attachments.ts (new), tests;
docs/data-entry/api-reference.md.

**Alternatives rejected:** enforce only in attachment.Service (doesn't cover
web, which bypasses the service); store-only (opaque error at HTTP boundary).
Chose ingress+backstop per decision.

## Security Considerations

- [x] Inputs, validation, sensitive ops, error-leak

**Inputs/validation:** multipart file (capped at ingress + store backstop;
filename sanitized for storage key via existing storeutil/property checks);
property validated against declared `file` props (allowlist→404). entityID is a
store key, never an FS path. **Sensitive ops:** write gated by `update` ACL
re-authorized server-side BEFORE bytes; deny audited. No `_actions` trust
(server re-authorizes). Orphan-avoidance ordering (authorize-before-write;
delete clears property first). 413/422 via problem+json; backend errors logged
not leaked.

## Test Plan

- [x] Scenarios, edge cases, negatives, integration

**Scenarios:** upload→200 + bytes round-trip via GET; replace overwrites;
delete→204 + GET 404; over-limit→413 (handler) + per-backend store-guard unit
test (fs/mem/pg); no-update-access→403 before any AttachFile (assert no bytes);
validation-fail→422; method probes. Frontend: widget upload/replace/remove,
gated on _actions, inline error on 413. **Edge:** empty file; missing form
field→400; non-file property→404; concurrent (writeMu). **Negatives:** oversize,
denied, malformed multipart.

## Risk Assessment

- [x] Risks + effort

**Risks:** store-guard change must pass storetest conformance (add a case).
Orphan ordering — mitigated by authorize-before-write. Frontend multipart
progress — keep simple (no chunking). Effort: l.

## Documentation Planning

- [x] Docs identified

- [x] docs/data-entry/api-reference.md — document upload/delete methods + 413/size limit on the Attachments section. Docs-checklist at review.
- [x] Configurable limit → note in config docs if present.

## Design Review

- [x] ~~/design-review~~ (N/A: extends the just-reviewed read-path patterns; the one genuinely new design call — Lua veto scope + size-limit seam — was decided with the user up front. Cranky /code-review at review stage covers implementation.)

**Design Review Findings:** scoping decisions made with user (Lua veto →
TKT-40PZ15; size limit → ingress+backstop).
