---
id: TKT-FJG779Q
type: ticket
title: 'List toggle cells: widget toggle with on/off values'
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
status: in-progress
---

## Description

A list column can declare `widget: toggle` with `on:` and `off:` values for an
enum or bool property. The cell renders a checkbox; clicking it writes the other
value through PATCH. An enum with values other than on/off needs `other:
disabled|hidden|on`, or the config fails to load. Each row carries whether the
toggle is writable for the principal, computed in one batch per page and pinned
by a budget test.
