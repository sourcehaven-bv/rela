---
id: TKT-ULCZ
type: ticket
title: Add file import and dark mode editing to palette settings
kind: enhancement
priority: medium
effort: m
status: done
---

# Add File Import and Dark Mode Editing to Palette Settings

## Problem

The palette import currently only supports paste into a textarea. Users
typically download palette files (.gpl, .hex) or share rela palette.yaml files —
they need drag & drop and file picker. Also, dark mode colors are auto-generated
but users can't see or tweak them in the Settings UI.

## Scope

1. **File picker button** — opens native file dialog to load .gpl, .hex, .txt, .yaml palette files
2. **Drag & drop** — drop palette files onto the import area
3. **Accept rela palette.yaml** — parse and import palette.yaml files for sharing between projects
4. **Dark mode toggle** — show light/dark palette side-by-side or with a toggle in the Appearance section
5. **Dark mode editing** — let users tweak individual dark mode colors (override auto-generated)
