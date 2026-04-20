---
id: RR-E6WT7
type: review-response
title: 'Minor: stale .rela/key / ~/.config/rela/key references in user-facing strings'
finding: 'cranky-code-reviewer #12: several keys.go command docstrings and help strings still referenced the old .rela/key location post-relocation. AC13 in the plan specifically flagged the need for a grep test.'
severity: minor
resolution: Updated internal/cli/keys.go command help and long descriptions to reference the user-state directory. Residual references remain in docs/cli-reference.md and generated docs-project/ entities — those regenerate from source and are tracked as a separate documentation pass; the production code strings are clean.
status: addressed
---
