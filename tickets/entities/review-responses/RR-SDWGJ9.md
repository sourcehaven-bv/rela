---
id: RR-SDWGJ9
type: review-response
title: 'Commit message understates severity: a freshly-initialized project failed to load entirely'
finding: 'FSLoader.Load (internal/metamodel/loader_service.go:57-68) treats migration detection as a hard gate, returning *migration.Error rather than warning. It is the main wiring path (internal/appbuild/appbuild.go). So a pre-fix `rela init` produced a project where EVERY command failed, not merely a noisy `migrate status`. Verified by building the parent commit (develop) and running `rela list requirement` in a freshly-initialized dir: `load metamodel: ... uses deprecated syntax`. The commit message described only the mildest symptom, which would mislead the next person triaging a similar report into thinking detection is advisory.'
severity: critical
resolution: 'Commit message rewritten to lead with the real severity: FSLoader.Load gates rather than warns, so a freshly-initialized project failed to load entirely and every command died. It also now records that the only available workaround (running rela migrate, as the error instructed) silently pinned the new project to legacy sequential IDs. The bug entity''s description and 5-whys were written from the verified behaviour.'
status: addressed
---
