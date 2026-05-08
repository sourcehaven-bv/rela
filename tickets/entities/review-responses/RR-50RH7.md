---
id: RR-50RH7
type: review-response
title: luaMdList doc comment still references old text-only shape
finding: Doc comment on luaMdList still says `{task=true, checked=<bool>, text=<string>}` and doesn't mention `inlines` or `children`. Code accidentally works for new shapes by passing items through verbatim.
severity: minor
reason: User-facing docs in lua-scripting.md cover the new shapes correctly. Internal-doc cleanup deferred to a follow-up.
status: deferred
---
