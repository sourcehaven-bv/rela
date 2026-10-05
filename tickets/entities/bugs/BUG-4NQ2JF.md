---
id: BUG-4NQ2JF
type: bug
title: Implicit default world exists beside declared worlds and shows no faced type
description: The unresolved default world is served when worlds are declared (landing world without default_world, and ?world=default), and it shows no faced type.
priority: medium
effort: m
status: backlog
---

## Problem

The implicit `default` world exists even when the operator declares worlds. It
applies no resolution: every type resolves to the zero face, which a faced type
has no row at (BUG-HC6I2T).

- Without `default_world`, a project with faces lands in the `default` world and every faced type shows nothing. `docs/content-states.md:565` calls `default_world` "effectively required" instead of making it so.
- `?world=default` stays selectable next to the declared worlds (`docs/content-states.md:576`), and it too shows no faced type.

## Expected

Per the generation principle (RES-Y6JA37): rela always has worlds. It generates
the `default` world only when the operator declares none; that world selects
every face of each type in declaration order. When worlds are declared, no
`default` world exists. A missing `default_world` is generated as the first
declared world.
