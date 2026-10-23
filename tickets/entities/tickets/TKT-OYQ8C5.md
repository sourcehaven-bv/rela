---
id: TKT-OYQ8C5
type: ticket
title: Fix mobile layouts in Atlas
kind: chore
priority: medium
effort: s
status: done
---

## Description

Correct mobile layout defects reproduced with the Atlas project: the navigation
drawer must scroll as a whole, the menu button must not overlap page titles or
remain visible above the open drawer, and mobile headers and create forms must
use the available viewport without clipping their content.

The create form editor also needs a usable full-width surface and scrolling
behavior on phones.

## Resolution

Updated the mobile shell, drawer, sticky topbars, create sheet, and detail field
layout. Verified the dashboard, navigation drawer, create form, and editor at a
390×844 viewport; the mobile browser suite passes.
