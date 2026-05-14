---
id: RR-DIL7
type: review-response
title: Tighten looksLikeSVG sniff to reject mid-document <svg>
finding: handlers_theme.go::looksLikeSVG (line ~183) does case-insensitive strings.Contains over the first 1 KiB. A polyglot beginning with `<?xml ?><!-- … --><svg>...</svg><script>...</script>` is XML-classified by DetectContentType, contains `<svg`, and gets served as image/svg+xml. CSP sandbox + nosniff neutralize this in the browser, but the server-side sniff is fragile. Tighten to require `<svg` to be the first non-whitespace, non-prologue, non-comment <…> element.
severity: significant
resolution: looksLikeSVG rewritten to walk past leading whitespace, UTF-8 BOM, XML prologue, doctype, and comments, then require `<svg` to be the first real element with a valid trailing char (whitespace, '/', or '>'). New TestSniffLogoMime_RejectsSVGPolyglots covers 9 cases including HTML wrappers, mid-document <svg>, text-prefixed <svg>, and `<svgfoo>` near-miss.
status: addressed
---
