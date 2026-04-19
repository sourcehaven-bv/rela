---
id: RR-4AXFJ
type: review-response
title: Untracked ticket files mixed into working tree with the rename diff
finding: The working tree contains 9 untracked files belonging to this ticket and its auto-created checklists (tickets/entities/tickets/TKT-MBX8V.md, planning/impl/review-checklist files, and 5 relation files under tickets/relations/). If git commit -a or a hook broadly adds files, the rename-only commit could either absorb the ticket bookkeeping (polluting the diff) or the ticket files could be forgotten entirely (the ticket entity would vanish from the repo). Commits should be one concept.
severity: significant
resolution: Renames committed separately from the ticket bookkeeping. Commit d92ad48 on branch fix/docs-project-plural-folders contains only the 34 R100 renames. The ticket/checklist/relation files will be committed as a second commit in the same PR so reviewers can see the rename diff in isolation.
status: addressed
---
