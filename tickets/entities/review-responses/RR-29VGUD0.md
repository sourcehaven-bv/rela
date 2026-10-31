---
id: RR-29VGUD0
type: review-response
title: Git status requests eat commands and faults
finding: Every store update sends a git status GET through the scheduler; aim faults at entity paths only and count the faults that hit a save.
severity: significant
resolution: The git store is mocked; faults apply only to entity requests sent by a tab, and only faults that hit count toward the floor.
status: addressed
---
