---
id: FEAT-870YCY
type: feature
title: Attachments as a first-class product feature
summary: 'Build the product surface on top of the existing attachment storage layer: web upload/download/preview (primary), MCP tool and CLI fixes (secondary), entity-inherited ACL, configurable per-property file count + sane default size limit, and file metadata exposed to Lua.'
description: 'Build the product surface on top of rela''s existing attachment storage layer. The storage layer (store.AttachmentManager, fsstore/memstore/pgstore backends, cascade-on-delete, CLI) is solid, but nothing a user touches exists: the web app renders file properties as plain text path strings, there is no upload/download HTTP endpoint (attachments are unreachable on postgres), no MCP tool, no ACL gating (NopACL), inconsistent/unbounded size limits, and a 1:1-per-property model that contradicts the CLI''s multi-file wording. Delivered via child tickets: web read path (primary, ships first), web write path (primary), configurable per-property file count, MCP tool (secondary), and CLI/docs cleanup (secondary). Cross-cutting decisions: attachments inherit the owning entity''s ACL; a sane default size limit is enforced in the shared write path for all backends; file metadata (name/size/content-type) is exposed to Lua as an escape hatch.'
priority: medium
status: proposed
---

## Problem

rela has a complete, well-factored attachment **storage** layer —
`store.AttachmentManager`
(`AttachFile`/`ReadAttachment`/`DeleteAttachment`/`ListAttachments`), three
backends (fsstore streaming, memstore, pgstore BYTEA), cascade-on-delete/rename,
and a CLI (`rela attach`/`attachments`/`detach`). What it lacks is **everything
a user touches**. Attachments are a file bound to a `file`-type entity property,
keyed 1:1 by `(entityID, property)`; the stored path string is stamped onto the
property value.

## Gaps this feature addresses

1. **No web path (primary).** The data-entry SPA renders a `file` property with a plain `TextWidget` (`frontend/src/widgets/registry.ts:100`) — the user sees the *path string as editable text*. No file picker, upload, download link, or preview. There is no HTTP endpoint for attachment upload/download. For a **postgres deployment the bytes live in the DB and are unreachable through the product** — this is the single highest-leverage gap.
2. **No MCP tool (secondary).** Agents cannot attach, list, or fetch files (`internal/mcp/` has nothing).
3. **No ACL (security).** Attachments are wired with `acl.NopACL{}` (`internal/attachment/attachment_test.go:92`). Entities/relations/search are ACL-gated in dataentry; attachment read/write is not. **Decision: an attachment inherits the ACL of its owning entity** (matches the 1:1 model, no new policy surface).
4. **Inconsistent size limits.** Only pgstore caps (64 MiB, `internal/store/pgstore/attachment.go:22`, self-described as backend-specific); fsstore/memstore are **unbounded**. **Decision: a sane product-wide default limit** enforced in the shared write path for all backends, sized for screenshots/PDFs (not movies). Tighter limits matter mainly for semi-untrusted users.
5. **Multi-file model mismatch.** CLI globs and says "Attach file(s)" but the model is 1:1 per property, so multiple files overwrite. **Decision: `file` properties gain a `max` setting (1..N)** — configurable count per property; store key scheme, metamodel, and backends change accordingly.
6. **Lua escape hatch.** Expose attachment file info (name, size, content-type) to Lua so users can implement their own constraints/policy via the existing write-veto hooks.
7. **CLI/doc cleanup.** Misleading multi-file wording; orphan window in `Service.Attach` (file written before `UpdateEntity`; `rela gc --temp-files` recovers); doc drift (`docs/metamodel.md:455` says `.rela/attachments/`, code uses root `attachments/`).

## Child tickets

- **Web read path** (ships first, independently useful): ACL-gated HTTP download/serve + inline preview / file widget replacing TextWidget. Unblocks *viewing* existing attachments, including postgres.
- **Web write path**: multipart upload endpoint + file-picker/drag-drop widget + default size limit in the shared write path + file info to Lua.
- **Configurable per-property file count (`max`)**: metamodel `file` property gains `max`; store key scheme + 3 backends + CLI move from 1:1 to N-per-property.
- **MCP attachment tool** (secondary): attach/list/read so agents participate, ACL-gated.
- **CLI + docs cleanup** (secondary): fix multi-file wording, document the model, orphan-window note, `.rela/attachments` doc drift.

## Out of scope

Magic-byte sniffing / MIME allowlist beyond what the size limit + Lua hook cover
(can be a follow-up if semi-untrusted multi-tenant lands).
