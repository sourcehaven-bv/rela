---
id: RR-CBM3TR
type: review-response
title: '`transition: all` animates the focus ring and any future property added to the rule'
finding: '`transition: all 0.15s` was copied from RelationCards. `all` catches `box-shadow`, so the focus ring fades in over 150ms rather than appearing on tab — a focus indicator that animates is a worse focus indicator. It also silently animates any property a future edit adds to the rule.'
severity: minor
resolution: 'Narrowed to `transition: background-color 0.15s, border-color 0.15s` — the two properties the checked-state change actually animates.'
status: addressed
---

The reviewer also raised `prefers-reduced-motion`. Not added: at 150ms on two
colour properties this is well inside the "not motion" category (no transform,
no position change), and the codebase has exactly one such guard
(`AutoSaveIndicator.vue:169`) on a genuinely moving element. Adding one here
would imply a convention the rest of the widget set does not follow. Recorded
rather than silently skipped.
