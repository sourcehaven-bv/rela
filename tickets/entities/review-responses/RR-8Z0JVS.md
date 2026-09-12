---
id: RR-8Z0JVS
type: review-response
title: Dedup key is a human-editable issue title
finding: 'Dedup matches on the issue title, which is mutable by the people being filed for. A maintainer renaming "Fuzz failure: FuzzGenerateShortID (internal/entity)" to something more descriptive breaks the match, and the next sweep files a duplicate indistinguishable from a genuine new find. Suggested alternative: embed a machine-readable marker (`<!-- fuzz-target: internal/entity FuzzGenerateShortID -->`) in the body and match on that, leaving the title free for humans to improve.'
severity: minor
reason: Real fragility, but the title format is set by this change and nobody has had a chance to rename one yet, so there is no live breakage to fix. Matching on a body marker needs body text in the list query (`--json body` over 200 issues, or a search), which is a heavier query than the current one and interacts with the page-limit concern in RR-SWV734. Better taken together with that, once the simple form has run against the real API and we know what the issues actually look like in practice.
status: deferred
---

Found by cranky-code-reviewer on the TKT-8LZGME diff (finding 11).

Mitigating factor for now: the titles are generated, and the workflow is the
only thing that files them, so they stay canonical unless a human edits one
deliberately. See [[RR-SWV734]] for the related query-shape question.
