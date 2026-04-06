---
id: TKT-K8UQ
type: ticket
title: Add customizable color palette to data-entry apps
kind: enhancement
priority: medium
effort: l
status: done
---

# Add Customizable Color Palette to Data-Entry Apps

## Problem

Users cannot customize the visual appearance of data-entry apps. CSS custom
properties are hard-coded in `App.vue` and badge colors are limited to 7 fixed
options. Different teams/projects may want distinct branding or color schemes.

## Scope

- Add a `palette` section to `data-entry.yaml` for defining custom colors
- Allow overriding accent color, semantic colors (success/error/warning/info), and background/text colors
- Support light and dark mode variants
- Expose palette customization in the Settings page for runtime changes
- Persist user palette preferences in `.rela/palette.yaml`
