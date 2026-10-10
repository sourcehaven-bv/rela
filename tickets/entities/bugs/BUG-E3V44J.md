---
id: BUG-E3V44J
type: bug
title: dataentry catch-all 422s send raw backend error text
description: Create, patch, put, relation, attachment, history-restore and view handlers answer any unclassified error as 422 validation_failed with err.Error(). A store or file-system fault reaches the client as a validation error with its text. Git sync does the same with a 200.
priority: medium
status: backlog
---

## Summary

Follow-up to BUG-Y34ZSZ (GitHub #1774), which fixed the 500 paths. The same
disclosure remains on catch-all branches that label every error a validation
failure and copy `err.Error()` into `detail`:

- `write_handler.go`: create, `ValidateCreate`, `writePatchError` (PATCH and
PUT), the `writeRelationsValidationError` fallback, create relation.
- `handlers_attachment.go`: `writeAttachmentWriteError` fallback, attachment
delete. These also log the cause.
- `history_restore.go`: restore fallback.
- `views_handler.go`: view execution.
- `handlers_git.go`: git sync puts the go-git error text in `resp.Error` with
a 200.

A pgstore fault during a PATCH answers 422 with the pq text (schema, table,
SQLSTATE). `CreateEntity` wraps store errors (for example `entitymanager: unique
check for %s: %w`), so they arrive here too.

## Fix direction

Send only typed client errors as 422 with their text
(`*entitymanager.ValidationError`, the face sentinels, structural errors,
`attachment.ErrRejected`), matched with `errors.As`/`errors.Is`. Send everything
else to `writeInternalError`. `copies_handler.go` already does this with a
sentinel allowlist. Each handler needs an inventory of what its callee can
return, which is why this was split from BUG-Y34ZSZ.

Extend `TestNoInternalErrorDetail` to the 422 catch-alls once they are
classified.
