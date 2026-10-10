---
id: BUG-Y34ZSZ
type: bug
title: dataentry 500 responses send the internal error text to the client
description: '21 dataentry handlers answered a 500 with err.Error() in the detail and did not log the cause. A store or database error text reached the client (GitHub #1774).'
priority: medium
effort: s
why1: Handlers passed err.Error() as the detail of a 500 and did not log the cause.
why2: writeV1Error takes a free-form detail string and the nearest error to hand was the one that caused the 500.
why3: 'The safe pattern (writeGateError and copy invoke: log the cause, answer check server logs) lived inline at a few sites and was never a named helper.'
why4: 'Nothing checked 500 responses for error text, so copies of the leaking line spread (the #1753 review counted 17 before it added 3).'
why5: The rule that a 500 detail must not carry internal text was implicit; there was no guard to make it structural.
prevention: writeInternalError / writeInternalJSONError are the named path, and TestNoInternalErrorDetail fails the build on any 500 call carrying err or .Error().
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Summary

GitHub #1774 (finding B2 from the security review on #1753). In
`internal/dataentry`, 21 sites answered a 500 with the raw error text in the
response (`err.Error()` in `detail`, or appended to the JSON `error`), and most
did not log the cause. A store or database error can name paths, tables or rows
the caller may not read.

## Fix

- `writeInternalError`, `writeInternalErrorDetail` and `writeInternalJSONError`
  log the cause with `slog.ErrorContext` and answer "check server logs". On
  `context.Canceled` they write nothing.
- The 20 leaking sites and six 500s that dropped the error (comments, feed)
  use them. The relation-write 500 and the 409 keep only request-derived
  fields (relation, op, target).
- Client errors that the 500 paths used to absorb now get their own status:
  clone validation is 422 `validation_failed`, an unresolvable conflict is 422
  `conflict_unresolvable`, a resolved conflict that fails validation is 422,
  and a delete race is the uniform 404.
- `TestNoInternalErrorDetail` fails on any call or composite literal naming
  `http.StatusInternalServerError` with a non-literal argument.

Follow-ups: BUG-E3V44J (the same leak on catch-all 422s and git sync) and
BUG-Q3Z15V (ungated conflict endpoints).
