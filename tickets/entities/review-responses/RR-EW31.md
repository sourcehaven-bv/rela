---
id: RR-EW31
type: review-response
title: rewriteEntityRefToken leaves stale `raw` field after mutating Codespan into Link
finding: 'frontend/src/utils/markdown.ts lines 92-97: the codespan token is cast `as unknown as Tokens.Link` and mutated in place. The function sets type/href/title/text/tokens but never touches `raw`. Marked''s Codespan has `raw: string` (the original `\`...\`` text); marked''s Link interface also has `raw: string` (the original markdown source like `[text](url)`). After mutation the token still carries the original codespan raw — e.g. `\`TKT-LXYHQ\`` — which is meaningless as a Link''s raw. Marked''s HTML renderer doesn''t currently use raw (it parses tokens), so today this is invisible. But any future walkTokens consumer, postprocessor, or marked extension that consults `raw` (e.g. for source-map purposes, source-reconstruction round-trips, or a future Markdown re-emitter) sees an inconsistent token. Also: the e2e snapshot of `rewritten` as `Tokens.Link` is structurally incomplete — marked''s parseInline calls the link renderer which destructures `{href, title, tokens}`, so practically this works, but the strict TS type contract is being violated by an unsafe cast. Either build a fresh Link object and `Object.assign(token, fresh)`, or be explicit about which fields are intentionally stale. At minimum, blank `raw` to `linkText` so it can''t surprise a downstream consumer with a stale codespan string.'
severity: minor
resolution: rewriteEntityRefToken now sets rewritten.raw = '[${linkText}](${href})' so the link-shape raw replaces the codespan-shape raw. Marked's HTML renderer ignores raw on links; defensive consistency for downstream re-emitters.
status: addressed
---
