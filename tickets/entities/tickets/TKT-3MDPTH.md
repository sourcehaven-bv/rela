---
id: TKT-3MDPTH
type: ticket
title: 'Decide what command output may reference: keep .rela/ secrets off every channel'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

A `commands:` script can name `.rela/secrets.yaml` in a `::rela::` `file`
message. It is inside the project root, so containment passes and it becomes a
downloadable token (TKT-93FUCV). Every principal holding that command's
`permission:` can then fetch the SMTP password and anything else stored there.

The root CLAUDE.md rule is "the configuration is not a secret; the data is",
with secrets called out separately: `.rela/secrets.yaml`, DSNs and tokens stay
off the wire. That rule has no representation on this path.

Raised as RR-U463I9 during the TKT-93FUCV security review.

## Why this is its own ticket

It is deliberately **not** a download-route patch. A script already has
arbitrary shell, so it can `cat .rela/secrets.yaml` into a `text` SSE message
today and reach the same readers. Adding a denylist to `mintFileToken` alone
would close the newest door and leave the older one open, while implying the
problem was handled.

The real question is what a command's *output* may reference, for every channel
it has. Decide it once:

- `file` messages (mint-time denylist)
- `text` / `message` payloads (harder: content, not a path)
- whether this is a denylist (`.rela/`) or an allowlist (an `out/` convention)

## Scope sketch

- Refuse to mint a download token for any resolved path under `<root>/.rela/`.
One comparison in `mintFileToken`, right after `containProjectPath`.
- Decide and document whether the `text` channel gets any equivalent, or
whether the trust boundary is explicitly "script authors are trusted, and the
config file is the boundary" — which is defensible, but should be written down
rather than left implied.
- Whichever way it lands, state it in `docs-project` so an operator writing a
backup script knows the rule.

## Acceptance criteria

1. A `file` message naming a path under `.rela/` mints no token; the item is
still listed without a download button.
2. A test pins that the bytes of `.rela/secrets.yaml` are never served by
`/api/command-file/`.
3. The `text`-channel decision is documented, whichever way it goes.
