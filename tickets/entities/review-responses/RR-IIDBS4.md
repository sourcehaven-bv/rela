---
id: RR-IIDBS4
type: review-response
title: The strict URI regex strips the renderer's own entity links
finding: '§0: entity-ID code spans become in-app links (today via `rewriteEntityRefToken` in walkTokens, i.e. BEFORE sanitizing), but `ALLOWED_URI_REGEXP` for http, https and mailto only removes every relative `/…` href, so those links disappear. Loosening the regex to admit relative URLs would also admit author-written `[x](/api/v1/...)` and `//evil` links. Fix: sanitize first with the strict regex, then turn the resolved-id code spans into links with DOM APIs on the sanitized tree (the same "post-sanitize, DOM-built, never concatenated" pattern as `wrapTablesForScroll`), building hrefs only from server-resolved RefProp ids and the page world. Add an AC 3 case: `[x](/entities/foo)` and `[x](//evil)` written by an author are stripped, while a resolved code span links.'
severity: significant
resolution: The renderer sanitizes first and then builds entity links with DOM APIs only from server-resolved ref ids, their face and the page world; author relative and protocol-relative links are stripped (R3).
status: addressed
---
