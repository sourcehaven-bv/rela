---
id: RR-EHZ9IQ
type: review-response
title: Forbid-list sanitizer still loads remote resources and lets messages restyle the app
finding: '§0 "Message markdown uses a strict renderer" promises "No remote image ever loads from a message", but it uses DOMPurify defaults plus `FORBID_TAGS`. The defaults still allow the `style` attribute (`<span style="background:url(https://evil/p.gif)">` is a tracking pixel that reveals each reader''s IP address and read time; `position:fixed;inset:0` draws a fake dialog over the whole SPA) and the `background` attribute (`<table background=...>` still loads in Chromium). `id` and `name` also survive and can clobber app anchors. Messages come from any member and from agents, so this is the main attack surface of the feature. Fix: use an allowlist, not a forbid list: `ALLOWED_TAGS` limited to p, br, strong, em, del, code, pre, blockquote, ul, ol, li, a, h1-h6, hr, table, thead, tbody, tr, th, td; `ALLOWED_ATTR` limited to href, title (plus `class` on code for highlighting, if needed); `SANITIZE_NAMED_PROPS: true`. Add AC 3 cases for `style="background:url()"`, `style="position:fixed"`, `<table background>`, `<a id=app>` and `<a ping>`.'
severity: critical
resolution: The message renderer uses an allowlist of tags and of the href and title attributes only, with named-prop sanitizing and a URI allowlist, so no style, background or remote fetch survives; AC 4 covers each case (R2).
status: addressed
---
