---
id: RR-JW2XT7
type: review-response
title: exportFilename picks a surprising extension for ambiguous content types (text/plain -> .conf)
finding: 'exportFilename (internal/dataentry/export.go) builds the download name with mime.ExtensionsByType(produces) and takes index [0]. That slice is alphabetically sorted, so text/plain returns [.conf .def .in .list .log .text .txt] and an export lands as "sales_review.conf". Surfaced during TKT-K7J6FL''s end-to-end verification, where a transform declaring produces: text/plain downloaded as .conf. PRE-EXISTING and shared by entity and list export: export.go has no changes on this branch, so all three export surfaces behave identically. application/pdf and application/vnd.oasis.opendocument.text - the realistic transform targets - resolve correctly to .pdf and .odt, so this only bites content types with many registered extensions. text/markdown returns an empty slice and falls back to a bare name with no extension at all.'
severity: nit
reason: 'Deferred: pre-existing and out of scope. internal/dataentry/export.go is unmodified on this branch, so entity, list and document export all share this behavior identically - it is not a regression. Fixing it inside a document-export ticket would silently change the download filename on two already-shipped surfaces. Not a security issue either: safeAttachmentFilename sanitizes the result regardless and the extension is cosmetic. The realistic transform targets (application/pdf, application/vnd.oasis.opendocument.text) already resolve correctly; only content types with many registered extensions are affected. Needs its own change with its own release note.'
status: deferred
---

## Why deferred rather than fixed here

Out of scope for TKT-K7J6FL and not a regression. `internal/dataentry/export.go`
is unmodified on this branch (`git diff develop --stat` is empty for it), so
entity, list and document export all share this behavior. Fixing it inside a
document-export ticket would change the download filename on two shipped
surfaces as a side effect, which belongs in its own change with its own note.

Not a security issue: `safeAttachmentFilename` sanitizes the result regardless,
and the extension is cosmetic.

## If picked up

A small allowlist of preferred extensions per content type, consulted before
falling back to `mime.ExtensionsByType`:

```go
var preferredExt = map[string]string{
    "text/plain":    ".txt",
    "text/markdown": ".md",
    "text/html":     ".html",
    "text/csv":      ".csv",
}
```

That also closes the `text/markdown` case, which returns an empty slice today
and yields a filename with no extension at all — arguably the worse of the two
symptoms, since a browser then has no hint what to open it with.
