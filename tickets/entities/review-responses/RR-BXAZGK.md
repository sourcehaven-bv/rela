---
id: RR-BXAZGK
type: review-response
title: .claude/* no longer ignores nested .claude directories
finding: A pattern with a slash is anchored; frontend/.claude/ became committable.
severity: significant
resolution: Restored .claude/ at any depth and re-included only /.claude/rules/ with anchored patterns; verified with git check-ignore.
status: addressed
---
