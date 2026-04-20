---
id: RR-SBJEV
type: review-response
title: 'Deferred: .rela/repo-id not auto-written to .gitignore at rela init'
finding: 'cranky-code-reviewer question: RR-LDRW3 said repo-id would be auto-gitignored alongside existing .rela/ entries. We implement the git-tracked check but not the proactive gitignore append.'
severity: minor
reason: 'Defense-in-depth: the post-hoc git-tracked check does fire when the file gets committed. Projects that ignore .rela/ (the default from workspace.Initialize) transitively cover repo-id. Adding a new gitignore append on top is a small feature bump not critical to the ticket''s threat closure.'
status: deferred
---
