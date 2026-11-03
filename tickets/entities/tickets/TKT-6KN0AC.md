---
id: TKT-6KN0AC
type: ticket
title: Lua rela.xml.decode and rela.feed.parse with hardened XML limits
kind: enhancement
priority: medium
effort: m
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

## Description

Give Lua scripts safe parsers for feed and XML payloads, so a script can consume
a feed without hand-written parsing. First of three steps for FEAT-A53JJF (see
TKT-RUO1GY); the declarative path will reuse this parser.

- `rela.xml.decode(string)` returns a plain Lua table.
- `rela.feed.parse(string)` parses RSS 2.0, Atom and JSON Feed into one
normalized item shape.

## XML safety

Build on `encoding/xml`: no external entities (no XXE), no DTD processing, only
the five predefined entities (no billion-laughs). Add an input size cap, depth /
element / attribute caps, `Strict = true` and a charset allowlist (UTF-8,
ISO-8859-1). Pin each with a hostile-input test.
