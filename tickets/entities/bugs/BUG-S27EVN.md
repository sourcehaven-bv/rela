---
id: BUG-S27EVN
type: bug
title: 'command: documents render hidden fields and share a cache across principals'
description: 'A command: document renders the raw entry including fields hidden from the reader, and its render cache key has no principal, so one principal''s render is served to others.'
priority: high
status: backlog
---

## Description

Found by the BUG-WJSYQT security review (not verified end to end).
`renderCommand` (internal/dataentry/document.go:543-554) reads the entry with
`s.store.GetEntity` on the raw store (wired at app.go:1258) and passes every
property, hidden ones included, to the render command. Any principal who passes
the row gate gets the HTML. The disk cache key `entryID-<FNV of raw entity>`
(:646-653, :240) has no principal and no ConfigID, so a render made for one
principal is served to every other.

## Fix

Read the entry through the visibility reader and pass the redacted entity to
`renderEntityMarkdown` and `computeDocumentHash`. Add a per-principal (or
per-verdict) component and ConfigID to the cache key. If a document needs raw
input, require `allow_acl_bypass`. Pin with a corpus case: a viewer opening a
command: document on an entry with a hidden field.
