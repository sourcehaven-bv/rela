---
id: TKT-C1ZVTT
type: ticket
title: 'Attachment CLI + docs cleanup: multi-file wording, orphan window, doc drift'
kind: docs
priority: low
effort: s
status: done
---

## Description

Small correctness/clarity fixes around the existing attachment CLI and docs.
These are independent of the larger web/MCP work and can land any time.

### Items
- **Multi-file wording.** `rela attach` accepts a glob and says "Attach file(s)" (`docs/cli-reference.md:315`), but with the current 1:1 model multiple files silently overwrite one slot. Until TKT-WLLRO7 (`max`) lands, make the CLI either reject >1 file for a `max: 1` property with a clear message, or document the overwrite behavior explicitly. After `max` lands, this wording becomes correct — coordinate.
- **Orphan window note.** `attachment.Service.Attach` writes the file before `UpdateEntity` (`internal/attachment/attachment.go:113`); a failed update orphans the file and the error tells the operator to run `rela gc --temp-files`. Document this in the CLI reference (and ideally tighten the ordering, but at minimum make the behavior discoverable). Note: the web write-path ticket should avoid reintroducing this.
- **Doc drift.** `docs/metamodel.md:455` says file attachments live in `.rela/attachments/`; the code uses root-relative `attachments/` (`internal/app/factory.go:71` `AttachmentsKey: "attachments"`). Fix the doc.
- **content_type dead column.** pgstore has a `content_type` column that `AttachFile` never populates (always `''`); content type is inferred at the service `List` layer from the extension. Either populate it on write or note it as intentionally derived. Low priority — capture as a code comment/decision.

### Acceptance
- CLI help and `docs/cli-reference.md` accurately describe the per-property attachment model.
- Orphan-recovery behavior is documented.
- `docs/metamodel.md` attachment path matches the code.

## Resolution (done)

Chose the documentation route for the multi-file item (the behavioral
`max`/reject change belongs to TKT-WLLRO7, which rewrites this CLI code anyway).

- **Multi-file wording** — `docs/cli-reference.md`: added an explicit note that passing multiple files/globs attaches them to the *same* property in sequence, each overwriting the last, with a forward-pointer to the `max` work (TKT-WLLRO7). CLI behavior left unchanged.
- **Orphan window** — `docs/cli-reference.md`: documented under `rela attach` that the file is written before the property update and a failed update leaves an orphan recoverable via `rela gc --temp-files`.
- **Doc drift** — `docs/metamodel.md:455`: `.rela/attachments/` → `attachments/`. Verified no other `.rela/attachments` references remain in `docs/`.
- **content_type dead column** — `internal/store/pgstore/attachment.go`: added comments at both the write site (column intentionally left at `''` default) and the read site (always `''`, content type derived at the service layer via `contentTypeForName`; selected for forward-compat only). Chose to document rather than populate, since deriving at write time would duplicate the service-layer logic. `go build -tags postgres ./internal/store/pgstore/` passes.

Parent: FEAT-870YCY.
