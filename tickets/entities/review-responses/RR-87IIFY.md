---
id: RR-87IIFY
type: review-response
title: Literal-only template never falls back to ID (undocumented corner)
finding: A template with literal text (e.g. "Mr. {achternaam}") with an empty placeholder renders "Mr.", never the ID, because only an all-empty result falls back. Follows the spec but is an unstated corner a reader might not expect.
severity: minor
resolution: 'Added a sentence to the metamodel guide''s Templates subsection: the ID fallback applies only when the rendered result is empty after trimming; a template with literal text always renders that text and never falls back.'
status: addressed
---
