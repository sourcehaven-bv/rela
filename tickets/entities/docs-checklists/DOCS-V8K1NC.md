---
id: DOCS-V8K1NC
type: docs-checklist
title: 'Docs: Lua scripts cannot distinguish an ACL-redacted property from a genuinely-unset one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported API
- [x] Non-obvious decisions explained in comments

`entity.Redacted` carries a full godoc: what it is, that it is a per-reader
artifact never persisted, that it deliberately does NOT feed `IsLocked()`, and
why disclosing names is intended.

`entity.InaccessibleReason`'s godoc was **corrected**: it previously invited
"Lua-driven access control" as a future reason, which is exactly the design this
ticket rejected. It now points at `Redacted` and explains the `IsLocked()`
consequence, so the next author is steered away from the trap rather than into
it.

`visibility.Redact` gained a stated PRECONDITION (RR-Q1VCKR): input must be a
raw store entity, never a prior `Redact` output.

## Project Documentation

- [x] `docs/lua-scripting.md` — new "Redacted properties" section
- [x] `docs/data-entry.md` — command-stdin payload contract
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`README.md`~~ (N/A: no project-level change)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern or convention; the existing
"configuration is not a secret" rule already covers the disclosure reasoning)

**Both edits were made to `docs-project/entities/guides/` and regenerated via
`just docs`.** `docs/*.md` carries `<!-- This file is auto-generated from
docs-project/entities/. Do not edit directly. -->` — an earlier revision of this
ticket edited the generated file directly, which would have been silently
reverted on the next regen. `just docs-check` verified in-sync.

## Content

- [x] Redaction semantics documented for script authors
- [x] Disclosure boundary stated explicitly (names, never values)
- [x] Runtime coverage table documented

The runtime table is the load-bearing part: `is_redacted()` is only meaningful
on the data-entry path. On the scheduler it is always `false` despite row gating
being active (pre-existing RR-7408F5 / RR-IHWEB0), and on CLI/MCP/docs no policy
is evaluated at all. Without the table a script author would reasonably read
`false` as "you are allowed to see this".

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: no changelog in this repo)
- [x] ~~API reference version bump~~ (N/A: additive to the domain type;
the v1 wire contract is unchanged — `_redacted` already existed via DEC-T0XIWQ
and is computed from verdicts, not from this field)
