---
id: RR-FLT6
type: review-response
title: Softbreak test coupled to marked.js implementation detail
finding: First test asserted textContent === 'foo\nbar' — but CommonMark allows softbreaks to render as either a line ending or a space. marked currently emits '\n' but goldmark/cmark may emit a space. The literal-'\n' assertion adds nothing the SPA actually cares about and would break on a marked upgrade.
severity: significant
resolution: Relaxed the assertion to whitespace-normalised text equality (`textContent.replace(/\s+/g, ' ').trim() === 'foo bar'`) and kept the structural `<br>` count = 0 check. The test now expresses the actual behavioural guarantee (no hard break, single paragraph) and is portable across renderers.
status: addressed
---
