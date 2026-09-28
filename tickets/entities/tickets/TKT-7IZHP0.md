---
id: TKT-7IZHP0
type: ticket
title: 'Generated faces and worlds: implicit face, generated default world only without declared worlds, faced write grants must name a face'
kind: enhancement
priority: high
effort: l
status: ready
description: 'Stage 3 of RES-Y6JA37: implement DEC-NPZICR''s generation rules for faces, worlds and write grants.'
---

## Description

Stage 3 of RES-Y6JA37: implement DEC-NPZICR's generation rules.

- A type declaring no faces has one implicit face at `""`, and every API treats it as a face.
- Record face declaration order at load (`FaceOrder`, following `PropertyOrder` at `metamodel/types.go:305`).
- When no worlds are declared, generate the `default` world: every face of each type in declaration order. When any world is declared, generate nothing: `default` and `?world=default` do not exist. A missing `default_world` is generated as the first declared world.
- An unqualified write grant on a type that declares faces is a load error; it must name `type@face`.
- Update `docs/content-states.md`, `docs/acl-security.md` and the SPA world switcher.

Bugs fixed on top of this: BUG-7MB1D5, BUG-4NQ2JF.

## Acceptance

- A project with faces and no worlds shows faced entities in lists, at the first declared face the reader may read.
- A project with declared worlds has no `default` world.
- `acl.yaml` with `write: [policy]` on a faced `policy` fails to load with a message naming `policy@<face>`.
