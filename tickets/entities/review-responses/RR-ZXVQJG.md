---
id: RR-ZXVQJG
type: review-response
title: Guard test walks the working tree
finding: Walking the disk counts untracked leftovers and ignored generated files; results differ between machines and CI.
severity: significant
resolution: repoFiles uses git ls-files --cached --others --exclude-standard.
status: addressed
---
