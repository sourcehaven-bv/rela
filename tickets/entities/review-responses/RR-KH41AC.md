---
id: RR-KH41AC
type: review-response
title: 'View-declared query_scope: is inert — nothing reads it at request time'
finding: 'dataentryconfig.List.QueryScope and Kanban.QueryScope are declared, documented, validated and folded into index derivation, but never read on any read path. The server honours only ?query_scope=, and the SPA sends it from nowhere (zero references to query_scope in frontend/). So an operator writing `query_scope: archief` on a list gets a config that validates clean, derives an index, and renders completely unscoped. The type''s `default` still applies (it is keyed off entity type, not the view), so the observable behaviour is ''default works, named scopes are silently inert'' — the exact fail-open shape the feature exists to prevent, relocated from the resolver into the wiring.'
severity: critical
status: open
---

## Status

Being fixed in this ticket, not deferred. AC4 ("a list or kanban that names no
scope returns only matching rows") happens to pass because it exercises the
default, which is type-keyed — so the acceptance criteria did not catch that the
*named* case never reaches the server.

## Why the gap exists

The transport decision was `?query_scope=<name>`, chosen because the list
endpoint is keyed by entity TYPE and the server therefore cannot tell which
configured list is on screen. That decision is sound and unchanged. What was
missed is that it makes the SPA a required participant: without a client that
attaches the parameter, `query_scope:` in `data-entry.yaml` is a key with no
reader.

The Go-side doc comment asserted the client behaviour in the present tense ("The
SPA attaches it from the list's `query_scope:` at the call sites that have
one"), describing code that did not exist. That is the doc-contradicts-code
drift that let two earlier fail-open bugs survive review in this same area.

## Fix

Attach the parameter in the SPA's list and kanban fetch paths, from the view
config the server already serves, and pin it with a frontend test asserting the
built query string. Also correct the Go doc comment to describe what is, and add
a server-side test that a named scope reaches the funnel.
