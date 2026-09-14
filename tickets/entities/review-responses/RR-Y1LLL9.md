---
id: RR-Y1LLL9
type: review-response
title: New function was inserted between probeAttachmentCommands' doc comment and its declaration
finding: 'warnIfScanCannotRun was inserted directly after probeAttachmentCommands'' godoc but BEFORE its func declaration, so probeAttachmentCommands now has no doc comment at all, and warnIfScanCannotRun''s doc opens with three lines describing a different function. Confirmed by reading handlers_attachment.go:260-277. gofmt is happy; go doc is not. Fix: move the whole warnIfScanCannotRun block (comment + func) below probeAttachmentCommands so each declaration reunites with its own doc.'
severity: minor
status: addressed
resolution: 'Moved the warnIfScanCannotRun block (interface, doc comment and function) ABOVE probeAttachmentCommands, reuniting probeAttachmentCommands with its own godoc. Confirmed by reading the file: each declaration now sits directly under the comment describing it.'
---
