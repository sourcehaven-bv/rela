---
id: RR-LELF94
type: review-response
title: 'Export path must never call GetCached: its key omits both ConfigID and principal'
finding: 'The plan asserts export does not reuse the entry-hash disk cache but gives a weaker reason than the real one. documentService.GetCached (document.go:234-240) keys on docCacheSubdir/<entryID>-<contentHash>.html with NO ConfigID and NO principal. Render''s singleflight key does include both (document.go:277, the RR-2QSGLU fix) but GetCached does not. It is safe today only because the cache is populated exclusively by command: renders (principal-independent) and the handler skips the read for script: docs (api_v1.go:2648). A shared gate helper that returned a cached result, or an export handler reaching for GetCached for symmetry, would reintroduce RR-2QSGLU on the export route. Since the plan refuses command: documents, the cache can never help export anyway. Separately, refresh and return_to/isSafeReturnPath are HTML-path-only and must stay out of the shared helper.'
severity: minor
resolution: Accepted. The extracted helper stops at the last gate and returns only the resolved render config plus the gated entity. refresh, return_to/isSafeReturnPath, GetCached, Render and RewriteDocumentLinks stay in the HTML handler. A comment at the export handler records that export must never call GetCached because its key omits both ConfigID and principal (RR-2QSGLU). Implementation pending.
status: addressed
---

## Verification

Confirmed. `GetCached`'s cache filename is built from `entryID` and the content
hash only — no `ConfigID`, no principal — whereas `Render`'s singleflight key is
`entryID + "|" + cfg.ConfigID + "|" + p.User + "|" + p.Tool`. The asymmetry is
real and is currently masked by the `docCfg.Script == ""` guard at the call
site.

## Resolution

Scope the extracted helper to **end at the last gate**. It returns the resolved
`documentRenderConfig` (plus the gated entity for the anchored case) and nothing
more. These stay in the HTML handler and must not migrate into it:

| Left behind | Why |
|---|---|
| `isSafeReturnPath` / `return_to` | Open-redirect guard for `RewriteDocumentLinks`; export emits a binary with no links |
| `refresh` query param | Cache control; export has no cache |
| `GetCached` | Key omits ConfigID and principal — RR-2QSGLU on a new route |
| `Render` / `RewriteDocumentLinks` | Produce HTML; export needs markdown |

Add a comment at the export handler stating export must never call `GetCached`,
with the reason (key omits ConfigID and principal), so the next person does not
add it "for symmetry".
