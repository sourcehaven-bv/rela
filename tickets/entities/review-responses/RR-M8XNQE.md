---
id: RR-M8XNQE
type: review-response
title: href and click agreed on destination but not on encoding
finding: |-
    Sharing `entityRowLocation` makes the click and the href agree on WHERE they
    go, but they disagreed on how that location is encoded, in two ways.

    (1) Path: `router.push({path})` treats its argument as a path and encodes it; a
    raw href is parsed by the BROWSER, where a `?` or `#` splits off a query or
    fragment instead of staying in the path. Entity ids cannot contain those (the
    backend constrains them), but `resolveLinkTarget` builds the path from a
    column's `link` config, which is operator-authored and unconstrained — so a
    document name with `?` in it gives a click and a Cmd+click that land in
    different places. That is precisely the divergence this ticket exists to remove.

    (2) Query: URLSearchParams serialises a space as `+`; vue-router emits `%20`.
    Both decode identically so navigation never breaks, but the address bar shows a
    different string depending on how the row was opened.
severity: significant
resolution: |-
    Added `encodeRoutePath` (percent-encodes each path segment, preserving the
    separators) and `encodeRouteQuery` (serialises the way vue-router does, so a
    space becomes `%20` not `+`). Both are used by `entityRowHref`.

    Deliberately NOT fixed inside `entityDetailHref`: that helper is shared with the
    command palette and its return value is consumed as a router `path` in some call
    sites and as a raw href in others. Encoding there would double-encode the path
    consumers. The encoding belongs at the point where a location becomes an href
    string, which is where it now lives.

    Two tests, both mutation-verified against a revert to the naive form:
    `encodes a space as %20, the way router.push would` and `percent-encodes a path
    that would otherwise split at a ? or #`.
status: addressed
---

## Resolution

Added `encodeRoutePath` (percent-encodes each path segment, preserving the
separators) and `encodeRouteQuery` (serialises the way vue-router does, so a
space becomes `%20` not `+`). Both are used by `entityRowHref`.

Deliberately NOT fixed inside `entityDetailHref`: that helper is shared with the
command palette and its return value is consumed as a router `path` in some call
sites and as a raw href in others. Encoding there would double-encode the path
consumers. The encoding belongs at the point where a location becomes an href
string, which is where it now lives.

Two tests, both mutation-verified against a revert to the naive form:
`encodes a space as %20, the way router.push would` and `percent-encodes a path
that would otherwise split at a ? or #`.
