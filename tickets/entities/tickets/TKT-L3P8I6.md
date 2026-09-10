---
id: TKT-L3P8I6
type: ticket
title: 'OBSOLETE: bare_face_changed no longer exists (bare_face removed in BUG-HC6I2T)'
kind: enhancement
priority: medium
effort: m
status: wont-fix
---

## Obsolete

Closed without work. BUG-HC6I2T (#1557) removed `bare_face` as a modelling key
entirely, so the `bare_face_changed` delta this ticket was about no longer
exists. `CompareShapes` now raises only `faces_introduced` and `faces_removed`.

The hazard it described — repointing which face the zero coordinate means, and
stranding rows at an occupied destination — cannot arise, because the zero
coordinate is no longer a face. A type with no faces stores its single state
there; a type with faces stores each row under its own face name.

The remaining faced-schema gap is `faces_removed`, tracked in TKT-1YBNQJ.

## Original description

Repointing `bare_face:` between two faces that both already exist could strand
data: the bare row would start impersonating the newly-bare face while the
pre-existing named row for it became unreachable. `rename_face` detected that
collision; `confirm_face` did not.
