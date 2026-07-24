---
id: RR-BM0KIJ
type: review-response
title: Stale exportRenderer comment describes a ?document= query override that doesn't exist
finding: The exportRenderer comment says 'A ?document=<name> override renders via a configured document' but the handler only consults the per-type export_render view config; no export handler reads a document query param. Pre-existing, but misleading about how the override is selected — fix the comment.
severity: minor
resolution: 'Comment rewritten: the override is the per-type export_render view config, config-selected only — no query parameter ever chooses the script.'
status: addressed
---
