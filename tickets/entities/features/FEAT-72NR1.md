---
id: FEAT-72NR1
type: feature
title: Unify widgets across form and view screens
summary: Make 'widget' a shared vocabulary across form and view screens, with a mode (display/edit/inline-edit) parameter. Forms default to edit, views default to display and opt into inline-edit per section. Unlocks Daily-Notes-style screens (task lists with click-to-toggle, inline-editable note bodies) as pure configuration.
description: |-
    Today rela has two parallel ways of rendering a property value: forms know how to edit values (with a 'widget' concept covering checkboxes, selects, dates, etc.), and views know how to display values (read-only, with one inline-toggle exception for markdown checkboxes).

    The two will be unified into a single widget vocabulary that works on both screen types. A widget renders one typed value in one of three modes -- display, edit, or inline-edit -- and the surrounding screen (form or view) picks which mode applies.

    Forms and views stay as distinct screen types -- they answer different user questions ('change this' vs 'understand this in context') and have genuinely different lifecycles. But the leaves of both -- the per-field renderers -- become the same library of widgets.

    The user-visible win: any property type rela understands (boolean, date, text, select, markdown) can be made inline-editable on a view screen by adding one config line. Today this requires bespoke Vue code per case (and only the markdown checkbox special case actually exists).

    This unlocks Daily-Notes-style screens (task lists with click-to-toggle checkboxes, inline-editable note bodies) as pure configuration, with no new screen-specific Vue components.

    Delivered in five independently-shippable increments; see implements tickets.
priority: high
status: proposed
---
