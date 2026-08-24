---
id: DOCS-CAP7YH
type: docs-checklist
title: 'Documentation: Lua capability gating (http, ai, secrets, write_file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`lua.Capabilities` carries the reasoning that is NOT recoverable from the code:
why the default is closed, why `Secrets` is a list rather than a bool, why
`AllSecrets` is a separate Go-only field instead of a `"*"` sentinel, and that
capabilities are ambient reach — NOT graph read-ACL, which stays
`ReadDeps.VisibleReader`. `metamodel.Capabilities` documents the YAML face and
the bool refusal. Both call-site comments in `mcp/tools_lua.go` say explicitly
not to "fix" a failing script by granting capabilities there.

## Project Documentation

- [x] README updated (if applicable) — N/A: no project-level surface change.
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern — this APPLIES the existing
      object-capability pattern that `rela.bypass_acl` established and that
      CLAUDE.md's "constructors reject nil required fields" / RR-X9NVHI
      fail-closed rule already state. Adding a rule would restate them.)
- [x] Help text accurate (if CLI changes) — N/A: no new or changed command.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no CHANGELOG file; release
      notes are derived from PR titles. The breaking change is called out in
      the PR body instead — see below.)
- [x] API docs updated (if applicable)

**User-facing docs updated:**

1. `docs-project/entities/guides/GUIDE-lua-scripting.md` → regenerates
   `docs/lua-scripting.md`. New "Capabilities" section covering the four
   capabilities, the `capabilities:` block, why `secrets:` is a list, why the
   default is closed, and the two surfaces that differ (operator shell grants
   everything; MCP grants nothing and cannot be configured to).
2. `docs/idp-webhook-provisioning.md` (hand-written, not generated). The shipped
   `examples/idp-sync.lua` walkthrough would have BROKEN without this — it uses
   `http` + two secrets. Its config block now shows the required
   `capabilities:` and explains that naming two keys withholds the rest of
   `.rela/secrets.yaml`.

Docs were edited at SOURCE and regenerated via `scripts/generate-docs.sh`;
`git status docs/` shows only the intended files.

**Breaking change (release note):** operators whose scripts use `http`, `ai`,
`rela.secrets` or `rela.write_file` on a config-declared surface (data-entry
action, webhook action, document, automation action, scheduled task) must add a
`capabilities:` block. The failure mode is loud and specific — `attempt to index
a nil value (global 'http')`, or a `nil` secret — never a silently-empty
credential. `rela script` / `rela flow` / the docs build are unaffected.
