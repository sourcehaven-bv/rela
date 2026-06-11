---
id: RR-7ALB6A
type: review-response
title: 'Guard diagnostics: collapse to one porcelain check with --untracked-files=all'
finding: Default porcelain output collapses untracked directories to a single entry (hiding which files were created), and the separate git diff + grep '^??' split was two code paths where one suffices.
severity: minor
resolution: Single `git status --porcelain --untracked-files=all` check now covers both modified-tracked and untracked files and lists actual file paths in the failure output. Also dropped the duplicate *.tsbuildinfo entry from frontend/.gitignore (root already ignores it repo-wide) and anchored the vite.config entries to the frontend root.
status: addressed
---
