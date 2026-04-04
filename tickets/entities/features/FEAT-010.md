---
id: FEAT-010
type: feature
title: Use goldmark for markdown rendering
summary: Use goldmark for markdown rendering with full GitHub Flavored Markdown support
description: Replace custom markdown parser with goldmark library for full GFM support including tables, task lists, and strikethrough
status: implemented
---

Replace custom markdown parser with goldmark library for rendering markdown content in the data entry web UI.

## Motivation

The custom markdown parser had limited support and required manual maintenance. Using goldmark with GFM extensions provides:

- Full table support
- Task list checkboxes
- Strikethrough
- All standard markdown features

## Implementation

- Use goldmark with GFM extension for parsing
- Post-process output for mermaid blocks and checkbox indices
- Add `md-table` class to tables for styling
