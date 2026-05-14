---
id: RR-VIWP
type: review-response
title: Consider lifting marked options to module-level const
finding: renderMarkdown is called per-render; the options object is reconstructed each call. Negligible perf, but it would make the 'this is our markdown dialect' decision a named, exported value (MARKDOWN_OPTIONS).
severity: nit
reason: Refactor out of scope for this xs bug-fix ticket. The options object is small and the walkTokens closure depends on per-call refResolver, so a useful module-level extraction would have to split static from per-call options — a more invasive change than warranted here. Can be a separate refactor ticket if the markdown dialect decision needs more visibility.
status: wont-fix
---
