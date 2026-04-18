---
id: RR-EGLZ9
type: review-response
title: FSStore.RenameEntity nolint:funlen removed but fn sits at statement limit
finding: In internal/store/fsstore/entity.go:353 the //nolint:funlen directive was removed (it is what nolintlint flagged as unused). The function is now exactly at the 60-statement limit; any future trivial addition (a log line, a defer) will re-trigger funlen. Either re-add the nolint with a comment, or plan to split the function at the next edit.
severity: minor
reason: 'The //nolint:funlen was auto-removed by golangci-lint --fix because nolintlint (v2) correctly detected the directive was unused: the function is at the limit, not over it. Re-adding a nolint preemptively is exactly what nolintlint was designed to prevent. If a future edit pushes the function over the limit, the natural response is to split it (the comment in the removed directive even acknowledged that). Accept the current state; let the next person hitting funlen make the split-vs-nolint call with fresh context.'
status: wont-fix
---
