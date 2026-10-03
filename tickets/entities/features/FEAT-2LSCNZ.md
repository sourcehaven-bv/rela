---
id: FEAT-2LSCNZ
type: feature
title: Agent guidance scoped to code paths
summary: Subsystem rules for coding agents load only when the agent touches the code they govern
description: 'Rules live in .claude/rules/*.md with paths: globs. Claude Code injects a rule when a matching file is read or edited. A guard test keeps every glob pointing at existing files.'
priority: medium
status: in-progress
---
