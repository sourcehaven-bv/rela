---
id: TKT-IO9PTLD
type: ticket
title: Link text and truncation for link properties
kind: enhancement
priority: medium
effort: s
started: "2026-11-19"
completed: "2026-11-19"
status: done
description: A string property with display link shows the raw URL as link text and a long URL runs over several lines. Add display link_text to set the text and cut a long URL off with an ellipsis.
---

## Description

A property with `display: {link: true}` shows its raw URL as the link text. A
long URL with a query string wraps over several lines in a detail row and
stretches table cells. There is no way to show a short label such as "Website"
instead.

## Acceptance criteria

- `display: {link: true, link_text: <text>}` shows that text as the link
on every surface; the URL stays the target and shows on hover.
- Without `link_text`, a long URL is cut off with an ellipsis on one line
and shows in full on hover.
- `link_text` without `link: true` is a schema error.
- Documented in the metamodel guide.
