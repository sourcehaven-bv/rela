---
id: TKT-YAROJX
type: ticket
title: 'MCP: attachment upload via signed upload URL'
kind: enhancement
priority: medium
effort: l
started: "2026-10-09"
status: wont-fix
---

## Description

An MCP client can attach a file only through `attach_file`, which takes the
whole file as base64 in the tool call. Every byte passes through the model's
context: a 300 kB screenshot is slow and costly, and one wrong character
corrupts the file. Source: Atlas TASK-2ZX3I (evidence screenshots for TASK-1EPRE
could not be moved between entities).

Apply the S3 presigned-PUT pattern:

1. **Issue.** New MCP tool `create_upload_url(id, property, file_name)`. It runs
the same preflight as `writePreflight` in `internal/mcp/tools_attachment.go`
(gated read, file property exists and is visible, entity not locked, ACL
`update`) and returns a URL carrying a token.
2. **Upload.** The client runs `curl -T <file> <url>`. The bytes bypass the model.
3. **Process.** The server receives the bytes over HTTP, streaming to disk, applies
the target property's upload policy (MIME filter, scan, max size) and writes
through `WriteAttachment`.

### Token requirements
- Bound to exactly one entity, one property, one file name, and the issuing principal.
- Single use; expires after a few minutes.
- ACL `update` is re-checked on use (rights may be revoked in between).
- Never appears in the access log (mask the query string or carry it in a header).
- Issue, use and refusal are audited, like `AttachmentWriteDenied` / `AttachmentRejected`.

### Out of scope
- Signed download URLs (possible follow-up, same mechanism).
- Removing `attach_file`; it stays for small files and clients without a shell.

### Acceptance
- An agent uploads a 400 kB screenshot with `create_upload_url` + `curl`, no base64 in the call.
- A token works once, not after expiry, and not once `update` is revoked.
- A file the property's MIME filter rejects is refused and audited.
- The token does not appear in server logs.

## Outcome

Won't fix: superseded by TKT-3DLP0K. A generic OpenAPI CLI (restish) with OAuth
reuses the existing JWT gate, ACL and upload policy, so no new token mechanism
is needed.
