---
id: RR-5Z3QBK
type: review-response
title: Code span across a soft line break may be matched on the server but not in the SPA
finding: 'mentions.codeSpanText concatenates Text segments and ignores the soft line break between them, so "`USR-\n7`" can yield "USR-7". CommonMark turns the line ending into a space, and the SPA uses Marked with breaks: true, so it shows "USR- 7" and never matches the chip. The plan''s edge-case list says such a span is "not one span, not a target", which the current code may not do. Fix: add the TestSpans row now. If goldmark gives two segments, Spans should refuse a span holding a soft or hard line break (or join with a space, as CommonMark does). This changes `All` slightly, so AC 1''s "unchanged" needs that exception noted.'
severity: minor
resolution: Spans refuses code spans holding a line break; TestSpans row added; AC 1 notes the one change to All.
status: addressed
---
