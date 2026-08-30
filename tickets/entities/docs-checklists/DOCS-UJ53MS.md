---
id: DOCS-UJ53MS
type: docs-checklist
title: 'Documentation: idp-sync claim validation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — for an example script the comment IS
the deliverable, so it carries three things the five lines of Lua cannot:

  1. **What it defends against** — a compromised or misconfigured IdP emitting a
subject containing `/`, `?`, `#` or a newline, which would reshape the outbound
request path or the entity filter.
  2. **What it is not** — defence in depth, not the primary control. The JWT is
already cryptographically verified (ES256, confused-deputy guard via a separate
audience). Without this sentence someone could read the regex as *the*
protection and relax the verification.
  3. **How to change it safely** — widen character by character, never to `.*`,
and specifically that Auth0's `auth0|abc123` is rejected by the default set.
Naming the realistic failure is what stops the first person who hits it from
reaching for `.*`.

- [x] ~~Function/type docs if public API~~ (N/A: a local Lua helper.)

## Project Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLAUDE.md updated~~ (N/A: no rela behaviour changes. The
allowlist-over-blocklist principle this follows is already in the project's
design-review guidance, with its own worked example.)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG.)
- [x] User-facing documentation updated — the script's own comment. That is the
correct and only place: `examples/` is documentation that happens to execute,
and a note in `docs/` telling readers the example validates its input would be
read by fewer people than the example itself.

The script's existing header already documents its params and secrets, so the
new comment sits where a reader adapting the file will encounter it — at the
guard, not in a preamble they skim.
