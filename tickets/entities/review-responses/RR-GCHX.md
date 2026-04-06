---
id: RR-GCHX
type: review-response
title: 'after mode recurrence: first run creates immediately'
finding: When recurring entity is first created with after mode, empty spawns list means task is immediately created. Also deleted tasks (not done) trigger re-creation.
severity: significant
reason: Immediate creation on first run is the intended behavior for after mode — the user just created a recurring template and wants the first task. Deleted-task re-creation is acceptable; if you don't want a task, set mode to exhausted or remove the recurring entity.
status: wont-fix
---
