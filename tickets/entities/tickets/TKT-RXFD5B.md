---
id: TKT-RXFD5B
type: ticket
title: 'Attachment web write path: upload endpoint + drag-drop widget + default size limit + Lua file info'
kind: enhancement
priority: high
effort: l
status: done
---

## Description

Let users **attach and remove files through the data-entry web app**, and fix
the inconsistent/unbounded size limits while doing it. Builds on the read-path
ticket (TKT-Q85275).

**Scope note:** the "expose file info to Lua" item was **split out to
TKT-40PZ15** after the TKT-Q85275 review established it's a net-new write-veto
mechanism (rela has no pre-write Lua veto; file size/content-type never reach
the entity write path), not a small binding. The product-wide default size limit
below covers the common case.

### Backend
- New data-entry **upload** (PUT/POST) and **delete** (DELETE) handling on the existing `_attachments` route, method-dispatched alongside the GET read handler. Upload writes via `store.AttachFile` then stamps the property + persists via `entitymanager.UpdateEntity` (mirrors `attachment.Service.Attach`); delete clears the property then `store.DeleteAttachment`.
- **ACL: inherit the owning entity's `update` permission.** Re-authorize **up front** via `translateVerb("update", type, id)` + `a.acl.AuthorizeWrite` (mirrors `authorizeConflictResolve`) so a deny happens *before* any bytes are written — avoids the orphan window. Read-gate first (uniform 404). Cover with an `acl_*` test. (Do not construct `acl.WriteRequest` directly — `lint_test.go` forbids it outside `affordances.go`; go through `translateVerb`.)
- **Sane default size limit, enforced ingress + per-store backstop** (decision): cap at the handler via `http.MaxBytesReader` (clean 413 as `application/problem+json` via `writeV1Error`), AND add a `maxAttachmentBytes` guard to fsstore + memstore so **no backend is ever unbounded** (pgstore already has one at `internal/store/pgstore/attachment.go:22`). Default ~screenshots/PDFs sized (start from pgstore's 64 MiB), made configurable via `dataentryconfig.Config` with a Go-constant default.

### Frontend
- File-picker + drag-drop on the file widget (extends the read-only `FileWidget` from TKT-Q85275): upload, show progress, replace/remove. Gate the controls on `entity._actions?.update !== false` (the SPA affordance convention). Surface server-side size/validation errors inline (reuse the 422/problem+json rendering).

### Acceptance
- Upload, view, replace, and delete an attachment end-to-end in the web app on fsstore and pgstore.
- A file over the default limit is rejected with a clear error on **all** backends (fsstore/memstore/pgstore), not just pgstore — handler returns 413, and the store guard rejects too (unit-tested per backend).
- A user without entity `update` access cannot upload/delete (acl test) — denied before any bytes are written.
- Orphan window closed for the web path: an ACL deny or validation failure on upload leaves no orphaned bytes.

### Notes / dependencies
- Multi-file (`max`) is a separate ticket (TKT-WLLRO7); build the widget/endpoint so adding N-per-property later is not a rewrite.
- Lua file-info veto: TKT-40PZ15 (depends on this ticket's upload handler as a host site).

Parent: FEAT-870YCY.
