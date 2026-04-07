---
id: BUG-0F0K
type: bug
title: 'Architecture lint fails: lua component missing runewidth vendor'
description: 'Architecture lint fails after PR #291 merged: lua component imports go-runewidth but .go-arch-lint.yml doesn''t declare it as an allowed vendor.'
priority: high
effort: s
why1: go-runewidth was added as a direct import in lua/markdown.go but not declared in .go-arch-lint.yml
why2: The arch config was not updated in the same PR that added the import
why3: The PR was auto-merged before the arch fix could be pushed
why4: There is no local pre-push step that runs the architecture lint before merging
why5: Architecture lint is only enforced in CI, so coupled changes (code + arch config) can land out of sync
prevention: Run just arch-lint locally before pushing
status: done
---

# Architecture lint fails: lua component missing runewidth vendor

## Problem

After merging PR #291 (GFM table parsing), the architecture lint check fails
because `internal/lua/markdown.go` imports `github.com/mattn/go-runewidth` but
the `.go-arch-lint.yml` config doesn't declare it as an allowed vendor for the
`lua` component.

## Fix

1. Add `runewidth` vendor entry in `.go-arch-lint.yml`
2. Add `runewidth` to `lua` component's `canUse` list
