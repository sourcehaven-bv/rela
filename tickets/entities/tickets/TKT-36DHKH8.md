---
id: TKT-36DHKH8
type: ticket
title: 'Lua rela.attach_file: store script bytes on a file property'
kind: enhancement
priority: medium
effort: s
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Description

A Lua binding that stores bytes a script already holds as a file on a `file`
property of an entity:

```lua
local att, err = rela.attach_file{
  entity = "INV-1", property = "pdf",
  content = resp.body, filename = "F001.pdf", mime = "application/pdf",
}
-- att = {name, size, sha256, mime}
```

Motivation: harbourmaster's invoice flow job fetches the invoice PDF from
e-Boekhouden with `http.request`, but can only keep the public `urlPdfFile`
link. `attach_render` stores only rela's own renders and `write_file` writes
only to the CLI output directory. With `attach_file` the job stores the PDF on
the invoice, and customers download it from rela under the ACL.

Behaviour, modelled on `rela.attach_render` (TKT-1TS55R):

- Runs as the script's principal, through the upload preflight (row gate,
field visibility, `update` write, `fields:` policy).
- Writes through the same path as an upload of the same bytes: size cap, MIME
allowlist, scan, count limit, entitymanager stamp, upload slot.
- `mime` is optional. When set, the bytes must sniff as that type and the
filename extension must belong to it (an extension is added when missing).
- Refusals return `nil, err`; only argument misuse raises.
- Available where `attach_render` is: data-entry actions, schedules, flow job
steps and background automations, in single-project and tenant mode. Not in the
synchronous cascade or document renders.
