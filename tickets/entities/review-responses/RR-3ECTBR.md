---
id: RR-3ECTBR
type: review-response
title: mimeCompatible signature carried both a derived claim and the filename it came from; extension normalized three times
finding: 'mimeCompatible(claimed, sniffed, fileName) took two different notions of what the file claims to be, with nothing in the signature saying which is authoritative for what: claimed is ignored entirely on the ZIP branch, so the inconsistent triple (''application/pdf'', ''application/zip'', ''x.docx'') returned true. Separately, strings.ToLower(filepath.Ext(name)) was computed three times per upload — in Process for the deny check, in mimeForExt, and again in mimeCompatible — so one normalization rule existed in three copies. The concern is not performance: whoever later handles a trailing space or a double extension will fix one copy and ship a bypass.'
severity: minor
resolution: Collapsed to extensionMatchesSniff(ext, sniffed), the shape the reviewer proposed. Process normalizes the extension once at the top and threads it to the deny check, the claim lookup and the ZIP decision, so the normalization rule exists in exactly one place; mimeForExt now takes the already-lowercased extension rather than re-deriving it from a filename. The no-claim short-circuit moved inside the function, next to the comment explaining it, so Process reads 'if !extensionMatchesSniff(ext, sniffed)' and the caller no longer has to know that an unknown extension means no check. The inconsistent-triple state is unrepresentable now that there is one input.
status: addressed
---
