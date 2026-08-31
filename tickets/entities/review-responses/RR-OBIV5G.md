---
id: RR-OBIV5G
type: review-response
title: Place the new WARN beside probeAttachmentCommands, which already walks the scan commands
finding: probeAttachmentCommands (internal/dataentry/handlers_attachment.go:264-289) already traverses meta.Attachments.ScanCmd plus every file property's ScanCmd and transform steps, warning per missing binary, and is already called from NewApp at the same startup point. The new 'configured but unsandboxed' WARN is the same class of boot-time diagnostic over the same traversal. Emitting it beside that call (rather than adding a second independent walk of meta.Entities) keeps the attachment startup diagnostics in one place and avoids duplicating the file-property traversal a third time. Not a correctness issue — placement/duplication only.
severity: minor
resolution: 'Plan updated: the WARN is emitted beside the existing probeAttachmentCommands(meta, runner) call, which already walks the global and per-property scan commands for its missing-binary warning. internal/dataentry/handlers_attachment.go added to the Files to modify list. Avoids a third traversal of meta.Entities and keeps attachment startup diagnostics in one place.'
status: addressed
---
