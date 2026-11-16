---
id: BUG-PT4FXRF
type: bug
title: remote_mcp config fails open on a null allow and on a decode error
description: 'An allow: with no value serves every tool, and refuseMisspelledRemoteMCP skips its check when a map decode of the file fails (sourcehaven-bv/rela#2324).'
priority: medium
effort: xs
why1: A null allow decoded to a nil slice, which the empty-list check (len 0 and non-nil) let through\; and refuseMisspelledRemoteMCP returned nil when its map decode failed.
why2: The empty-list check tested the decoded value, where null and absent look the same, instead of the YAML node. The spelling check decoded into a map, which fails on a duplicate key that the struct decode ignores, and assumed the main decode would report any such error.
why3: 'The tests covered allow: [] and a misspelled key in an otherwise valid file, but no null value and no file the two decoders read differently.'
why4: TKT-2A9VLGG added the strict decode with tests for each refusal it thought of, and the review checked inputs that were written down; nothing generated the YAML forms (null, alias, merge, extra document) that parse differently from what the code expects.
why5: Config checks in this repo are judged by example inputs, and YAML has many equivalent spellings for one value. A check that reads a decoded value inherits every one of them unless it is written against the node and tested against each form.
prevention: 'Both checks now read the YAML node: allow refuses a null scalar, and the spelling check walks the top-level mapping keys and returns a parse error. TestLoadConfig_RemoteMCP pins the null and duplicate-key cases (AM-remote-mcp-config-fail-closed).'
started: "2026-11-15"
completed: "2026-11-15"
status: done
severity: minor
---

## Description

Finding 1 of the security review of sourcehaven-bv/rela#2305, filed as
sourcehaven-bv/rela#2324.

1. `remote_mcp.tools.allow:` with no value (all entries commented out, or
`allow: ~`) decodes to a nil list and serves every tool, while `allow: []` is
refused.
2. `refuseMisspelledRemoteMCP` returned nil when decoding the file into a map
failed. A duplicate key in another section makes that decode fail while the main
struct decode passes, so a misspelled `remote-mcp: {enabled: false}` was
ignored.

## Expected

Both are refused at startup.
