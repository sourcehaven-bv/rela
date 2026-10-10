---
id: TKT-3DLP0K
type: ticket
title: OpenAPI spec complete enough to drive rela-server from restish (attachments + auth)
kind: enhancement
priority: medium
effort: l
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

Make a remote rela-server usable from a generic OpenAPI CLI such as
[restish](https://rest.sh), so an agent with a shell can work with a remote
project without passing data through the model. The motivating case is Atlas
TASK-2ZX3I: an agent must upload a 400 kB screenshot as an attachment, and the
MCP tool `attach_file` forces the model to write the whole file as base64.

With restish, the user logs in once (OAuth, token cached by restish), and the
agent runs a command such as `restish atlas put-task-attachment TASK-1EPRE
evidence <shot.png`. The bytes go from disk to the server; the model only writes
the command.

This replaces the signed-upload-URL approach of TKT-YAROJX: it reuses the
existing JWT gate, ACL, upload policy and audit, so it needs no new token
mechanism.

### Known gaps (from a first look)
- The spec (`internal/openapi`, served at `/api/v1/_openapi.json`) covers entity
CRUD, relations and clone per type, plus a few legacy `/api/` paths. It does not
describe attachments, views, actions, trace, history or comments.
- No security scheme or OAuth metadata is published, so a client cannot discover
how to log in. OAuth for Atlas is handled in front of rela-server.
- The attachment upload route takes multipart only.

### Scope
To be settled in planning. Attachments (list, download, upload, delete) and the
security scheme are the minimum.

### Acceptance (draft)
- restish, configured only with the server URL, logs in via OAuth and lists entities.
- An agent uploads a 400 kB screenshot to an entity through restish, with no base64 in the model's output.
- The upload goes through the same ACL, upload policy and audit as the web upload.
- The generated spec validates as OpenAPI 3.1.
