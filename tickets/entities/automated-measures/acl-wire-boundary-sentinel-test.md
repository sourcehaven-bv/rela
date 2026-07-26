---
id: acl-wire-boundary-sentinel-test
type: automated-measure
title: 'Wire-boundary ACL sentinel test: no hidden property value appears in any registered-route response'
kind: test
location: internal/dataentry (wire-boundary test; route-enumerated)
status: proposed
description: >-
  A single test that sets an ACL policy hiding a property whose value is a
  distinctive sentinel, then walks every REGISTERED ROUTE and asserts the
  sentinel bytes appear nowhere in the raw response. Route enumeration (not a
  hand-listed set) is what makes it cover future read surfaces automatically;
  pair it with a registry check that fails on any route neither exercised nor
  explicitly exempted with a reason. Byte-level scanning sidesteps the
  incompatible-DTO problem and covers both the property-value variant
  (BUG-9QL9XV) and the title variant (BUG-R9EHKV) with one assertion, because it
  does not care which field carries the bytes.
---

## Why one measure for both bugs

BUG-9QL9XV (hidden values via `_views` property payloads) and BUG-R9EHKV
(hidden values via `DisplayTitle` titles) are two variants of one class: a
hidden property's value reaching the wire through a surface that forgot to
strip. A byte-level sentinel scan detects both without enumerating fields or
DTOs, so a single guard is the correct granularity — duplicating it per bug
would be two copies of the same test.

## Status

`proposed` — the BROAD, route-enumerated sentinel test does not exist yet.
A narrower per-surface regression test now covers the `_views` path
(`TestACLViews_RedactsHiddenPropertyValue` / `...PrimaryTitle`), verified to
fail without the fix — but that is a hand-listed surface, exactly the decay the
route-enumerated version is meant to prevent. Promote to `active` only when the
registered-route sentinel scan lands, not before, so the graph does not claim a
surface-agnostic guard that isn't there.

