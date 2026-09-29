---
id: BUG-OJPVPG
type: bug
title: Search ranks faces before the face gate, so a face-restricted reader gets the wrong hits
description: Search resolves each entity's world prime over every face and only then drops withheld faces. A type@face reader misses entities the list shows, and list ?q= matches text of a face the reader may not read.
priority: high
status: backlog
---

## Description

Search backends resolve the world prime over all faces of an entity. The face
gate from a `type@face` grant runs afterwards (`faceGatedHits` in dataentry, the
gated reader filter in MCP). The read contract elsewhere is filter-first: the
ACL trims faces, then the world ranks what is left (`internal/worldreader` guard
rule 1).

Consequences, all confirmed in the BUG-6XTX0G and BUG-SMPOZB security reviews:

- `_search?q=<term>` and MCP `search_entities` drop an entity whose prime is a withheld face, although the list and the entity GET serve its readable face. Comparing the two tells the reader that a hidden, higher-ranked face exists.
- The list `?q=` path (`freeTextIDs`) matches the text of the withheld face and intersects ids with a face-trimmed list, so it returns the readable face for a term that appears only in the hidden face. That is a value oracle on withheld-face text.
- The backend `Limit` applies before the gate, so hits can starve.

## Fix

Carry the face allowlist on `search.Query` (an allowlist, compiled from grants)
and honour it in every backend (bleve, linear, pgstore `worldSQL`, sqlite)
before ranking. Keep `faceGatedHits` as a backstop.
