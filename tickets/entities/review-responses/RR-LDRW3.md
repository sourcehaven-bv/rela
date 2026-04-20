---
id: RR-LDRW3
type: review-response
title: .rela/repo-id could be committed to git, leaking cross-collaborator state-dir collisions
finding: If .rela/ isn't gitignored (user explicitly un-ignored, or committed config to share cache), .rela/repo-id lands in tracked files. Every collaborator then shares a user-state directory on their own machine, cross-contaminating ui-state/palette/defaults. Worktrees copied to sibling dirs collide. Plan's 'documented' mitigation is abdication.
severity: critical
resolution: 'On load, check if .rela/repo-id is git-tracked (git ls-files --error-unmatch). If tracked → error with clear instruction to gitignore it. File contents include a ''# DO NOT COMMIT'' header comment. On git-unavailable (no .git dir) skip the check. Also: .gitignore entry for .rela/repo-id gets written on rela init alongside existing .rela/ entries.'
status: addressed
---
