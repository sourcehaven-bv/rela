---
id: TKT-1TS55R
type: ticket
title: 'Lua rela.attach_render: render, convert and attach'
kind: enhancement
priority: medium
effort: l
started: "2026-10-27"
completed: "2026-10-28"
status: done
---

## Description

A Lua binding that renders a data-entry document or a list export as the calling
principal, converts it through a registered transform, and attaches the result
to a file property of an entity. Usable from actions, automations and schedules;
not from document renders or validation, which cannot write.

```lua
local att, err = rela.attach_render{
  entity   = "AUD-2026-1",      -- target entity (addressed like other write bindings)
  property = "rapport",         -- a file property on that type
  document = "soa",             -- a data-entry document, or list = "<list id>"
  entry    = "KADER-ISO27001",  -- the document's entry entity, if it takes one
  format   = "pdf",             -- a registered transform name, or "md"
  filename = "SoA-2026-10-27.pdf",
}
-- att = { name=, size=, sha256=, mime= }
```

## Requirements

- The render runs as the calling principal and sees only what that principal
may read, exactly like an interactive export (no elevation).
- The attachment write goes through `entitymanager.Manager` with the normal
write ACL and audit; a principal who may not write the file property gets an
error.
- The transform runs through the existing transform engine and `cmdexec`
confinement; the attachment scanning and MIME rules apply as for an upload.
- SHA-256 of the stored bytes is computed, recorded with the attachment and
shown in the UI and API attachment metadata (for every upload if simple; closes
design-doc gap G13).
- Errors are returned (`nil, err`), never swallowed.
- Docs in the Lua scripting guide and the transforms guide.

## Acceptance criteria

- An action on an `audit` entity attaches a PDF of a document.
- The recorded hash matches the stored bytes.
- A principal without read access to some rows gets a PDF without them.
- A principal without write access to the property is refused.
- Works on fs, SQLite and PostgreSQL.

Brief: G5 freeze and baselines, ticket 1.
