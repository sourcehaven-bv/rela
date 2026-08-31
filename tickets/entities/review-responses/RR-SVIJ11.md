---
id: RR-SVIJ11
type: review-response
title: Gantt handler must be an extracted struct, not App methods (plimsoll cap)
finding: 'The plan lists `internal/dataentry/gantt_handler.go` but does not say the handler must be its OWN struct with its own collaborators. `App` carries `//plimsoll:max-methods=104` (internal/dataentry/app.go:112) and the code comments state the type is AT its load line — app.go:403 explicitly makes appRedactor a package function rather than a method because ''the type is at its load line ... DRY here must not cost a ratchet''. Adding gantt handler methods to App fails the God-object lint CI job. The codebase already has the sanctioned pattern: attachmentHandler, exportHandler (extracted in TKT-JF5JI8 precisely ''to keep App under its plimsoll method cap''), writeHandler, viewsHandler (app.go:218-232). The plan must specify a `ganttHandler` struct holding its own narrow consumer-side interfaces, wired in like `views`/`export`.'
severity: significant
resolution: 'Plan updated: the Approach section now specifies ''The handler is its own struct, not App methods'', citing //plimsoll:max-methods=104 (app.go:112) and the two places the codebase treats it as a hard ceiling (app.go:403 appRedactor-as-package-function, :969). Prescribes following viewsHandler/exportHandler (app.go:218-232) with a ganttHandler holding narrow consumer-side interfaces, read-only and taking no writeMu — matching viewsHandler, which is documented as ''No writeMu — this surface never mutates''. Files-to-modify updated to wire it near app.go:218-232.'
status: addressed
---

## Finding

The plan's "Files to modify" names `internal/dataentry/gantt_handler.go` but is
silent on whether the handler is a struct or methods on `App`. Left unstated,
the mechanical implementation would hang them off `App` and fail CI.

`App` is grandfathered at `//plimsoll:max-methods=104` (`app.go:112`), and the
surrounding code treats that as a hard ceiling, not a budget:

- `app.go:403` — `appRedactor` is deliberately a package function, not a
method, because "the type is at its load line — see the struct doc; DRY here
must not cost a ratchet".
- `app.go:969` — another closure justified by "App is at its plimsoll method
load line".

## Existing pattern to follow

`app.go:218-232` shows four handlers already extracted for exactly this reason:

- `attachments *attachmentHandler` (TKT-R68TV8)
- `export *exportHandler` — "Extracted from App (TKT-JF5JI8) to keep App under
its plimsoll method cap"
- `write *writeHandler`
- `views *viewsHandler` — "No writeMu — this surface never mutates"

## Resolution

Specify a `ganttHandler` struct in the plan, owning the tree build and roll-up
fold, holding narrow consumer-side interfaces (per CLAUDE.md's
define-interfaces-at-the-call-site rule) rather than a reference to `App`. Like
`viewsHandler`, it is a read-only surface and takes no `writeMu`.
