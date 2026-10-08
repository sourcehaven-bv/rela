---
id: RR-V4CQ4S
type: review-response
title: Rule files in subdirectories are not checked
finding: filepath.Glob on *.md is not recursive.
severity: minor
resolution: ruleFiles walks .claude/rules recursively.
status: addressed
---
