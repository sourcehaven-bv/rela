---
id: RR-FXWM9M
type: review-response
title: Hidden presentation fields are erased by an edit
finding: 'dataentryconfig/channels.go introduces `hidden: true` for an optional property, and EditMessageRequest replaces EVERY prop (apiwire/v1/channels.go, api/channels.ts editMessage). The composer never renders a hidden field, so it never sends it back. Any value set by a trigger, a system post, MCP or the CLI is therefore silently deleted the first time the author edits the message in the SPA. This is the same data-loss class R7 handled for restricted refs, reintroduced by this PR''s config surface, and it is the read-modify-write erasure the root CLAUDE.md warns about for entities. Fix now, in T7a: either define that edit mode carries every hidden (and every presented-but-unrendered) prop through unchanged (fromPropValue -> toPropValue pass-through in channelValues.ts, with a test), or refuse `hidden` on a property while edit_own is offered. Document the choice in docs/data-entry.md next to `hidden`.'
severity: significant
resolution: The SPA edit path (toEditProps) carries every unrendered or hidden prop back as stored, the server keeps unnamed stored refs the caller cannot read, and both guides document that an edit never erases them.
status: addressed
---
