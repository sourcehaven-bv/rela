---
id: DEC-NPZICR
type: decision
title: rela always has faces and worlds; omitted configuration is generated, explicit configuration replaces it
context: 'Face support was added route by route to APIs built for one record per id. Faced types have no zero-face row (BUG-HC6I2T), so every surface that reads the zero face fails quietly: 41 face-blind surfaces in the inventory and a recurring bug class (BUG-64MU2Q, BUG-OOZBBK, BUG-VFHUWO, BUG-R1PQY9, BUG-CTUW2N). The default world resolves faced types to that missing row, and a type-wide write grant authorizes only it.'
consequences: Every type has at least one face, so face-level APIs take a typed address and no API can read the zero face of a faced type. The default world is generated only when no worlds are declared and then shows faced types, at the first declared face the reader may read. Declaring any world removes default and ?world=default. An unqualified write grant on a faced type fails at load instead of silently authorizing nothing. Implementation is staged in RES-Y6JA37.
date: "2026-09-28"
status: accepted
---

## Decision

rela always has faces and worlds. Configuration for them may be omitted as a
convenience for simple projects. rela then generates exactly the configuration
the operator would otherwise have written, and uses it. Once the operator writes
that configuration, it is used as written; nothing is generated or mixed in.

Rulings:

1. **Faces.** A type that declares no faces gets one implicit face, stored at the existing `""` coordinate, so no data moves and faceless addresses stay `ID` on the wire. A type that declares faces has exactly those.
2. **No primary face.** Choosing a face is a world concept. A type declares its faces, not a preference among them.
3. **Worlds.** When no worlds are declared, rela generates one world, `default`, that selects every face of each type in declaration order. When any world is declared, nothing is generated and no `default` world exists. A missing `default_world` is generated as the first declared world.
4. **Write grants.** An unqualified write grant on a type that declares faces is a load error; the grant must name `type@face`. A type-wide read grant keeps covering every face.

The research, options and staging are in RES-Y6JA37.
