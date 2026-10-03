---
id: TKT-1U70O4
type: ticket
title: Path-scoped agent rules instead of one large CLAUDE.md
kind: chore
priority: medium
effort: m
started: "2026-10-03"
status: in-progress
---

## Description

The root `CLAUDE.md` loads in full for every agent, although most of its rules
govern a single subsystem. Claude Code loads `.claude/rules/*.md` files with a
`paths:` frontmatter only when a matching file is read or edited, including in
subagents.

Move subsystem-specific rules out of the root `CLAUDE.md` into path-scoped rule
files, keeping cross-cutting rules in the root. Add a guard test that fails when
a rule's glob matches no file, so a moved package does not silently orphan its
rules.
