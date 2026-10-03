---
id: PLAN-35FUD4
type: planning-checklist
title: 'Planning: Path-scoped agent rules instead of one large CLAUDE.md'
started: "2026-10-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: move subsystem-specific rules from the root `CLAUDE.md` into
`.claude/rules/*.md` with `paths:` globs, keeping every line of text; keep
cross-cutting rules and the architecture overview in the root; track
`.claude/rules/` in git; a guard test for the globs. Out: rewriting rule
content, the nested `CLAUDE.md` files, and the managed `.claude/agents` and
`.claude/commands` files.

**Acceptance Criteria:**

1. Every non-blank line of the old root `CLAUDE.md` appears in the new root or
in a rule file (scripted line check).
2. Each rule file has a `paths:` list, and each glob matches at least one file
(`TestRulePathsMatchFiles`); a broken glob fails the test.
3. The glob translation handles `**`, `*`, `?`, braces and classes
(`TestGlobRegexp`).
4. `.claude/rules/` is tracked; other `.claude/` content stays ignored.
1. ...

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: uses a documented Claude Code feature; no options to weigh)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:**

N/A

**Existing Solutions:**

Claude Code path-scoped rules (`.claude/rules/*.md`, `paths:` frontmatter)
load on Read/Write/Edit of a matching file and work in subagents. Nested
`CLAUDE.md` files (already used by `frontend/`, `internal/dataentry/`,
`internal/entitymanager/`) load per directory but cannot cover a rule that
spans several packages, such as versioning. A PreToolUse hook could inject
context but duplicates the built-in feature.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

A script moves line ranges verbatim and demotes section headings. The root
keeps short pointers for the condition-engine, transforms and storage sections
and gains a table of rule files. The guard test parses the frontmatter, turns
each glob into a regexp and walks the repository (skipping `.git`,
`node_modules`, `.ignored`, `build`).

**Files to modify:**

`CLAUDE.md`, `.claude/rules/*.md` (new), `.gitignore`, `tools/agentrules/` (new).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Repository files only; no runtime input.

**Security-Sensitive Operations:**

None. The moved rules include security rules (ACL ceiling, mail
sanitizing); they now load when the governed code is touched, which is when
they apply.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

See acceptance criteria.

**Edge Cases:**

A glob with braces or classes; a rule file without frontmatter; a moved package.

**Negative Tests:**

A misspelled glob fails `TestRulePathsMatchFiles` (checked by hand).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

An agent working through a tool other than Read/Write/Edit (grep only, or a
non-Claude agent) does not get the rule loaded. Mitigation: the root table lists
every rule file and says to read it directly.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal agent instructions)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore ticket)

**Documentation Impact:**

The root `CLAUDE.md` documents the rule files. No user-facing docs.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: documentation move plus a test; no design surface)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review, see above)

**Design Review Findings:**

N/A
