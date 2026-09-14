---
id: BUGA-3IPX4M
type: bug-analysis-checklist
title: 'Analysis: ZIP-container documents (.docx, .xlsx, .odt, .epub) rejected on upload: mimeCompatible does not tolerate a sniffed application/zip'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Built a real ZIP with `archive/zip` and ran it through
`newMIMEProcessor(nil).Process` under each extension. All seven container
formats were rejected; `.zip` passed:

```
a.docx: file extension implies "application/vnd.openxmlformats-officedocument.wordprocessingml.document" but content is "application/zip"
a.xlsx: ... spreadsheetml.sheet ... but content is "application/zip"
a.pptx: ... presentationml.presentation ... but content is "application/zip"
a.odt / a.ods / a.odp: ... vnd.oasis.opendocument.* ... but content is "application/zip"
a.epub: ... "application/epub+zip" ... but content is "application/zip"
a.zip:  (passes — claim and sniff both application/zip)
```

Environment: darwin, Go stdlib `http.DetectContentType` +
`mime.TypeByExtension`. Not host-specific — the ZIP magic-byte rule is in the
stdlib sniff table.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

`mimeCompatible` tolerated `application/octet-stream` as the only generic sniff
result. `application/zip` is the sniffer's other generic result, and
ZIP-container documents are exactly the case it covers.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: an explicit `zipContainerMIMETypes` allowlist of types that
legitimately ARE ZIP archives, tolerated in `mimeCompatible` only when the sniff
is `application/zip`. Deliberately not a blanket "any claim over ZIP bytes" rule
— ZIP also carries the executable formats `.jar`, `.apk` and `.xpi`, where the
extension claim is what the mismatch check should keep rejecting.

Related areas checked: `deniedExtensions` / `deniedMIMETypes` are unaffected
(neither mentions ZIP). The `DefaultSafeMIMETypes` doc comment asserted the same
wrong belief and is corrected. The published guide said "office documents sniff
this way" of octet-stream; corrected in `docs-project/`.
