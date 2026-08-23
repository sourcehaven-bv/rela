---
id: AM-delete-versions-every-face
type: automated-measure
title: Deleting an entity records a delete version for every face
description: A test that Manager.DeleteEntity and RenameEntity capture one version per state row, not one for the default face. Guards the class of bug where a bare-id read feeds a capture while the store sweeps the whole state family.
kind: test
location: internal/entitymanager/manager_test.go
status: proposed
---

## What it guards

The write choke-point reads the entity with `GetEntity(id)` — the DEFAULT face
— and captures one version from it, while `store.DeleteEntity` hard-deletes the
whole family. The asymmetry is invisible at the call site because both take a
bare id; nothing in the signature says one is per-face and the other per-family.

`RR-181AFY` closed exactly this for cascade-deleted **relations**, and the loop
it added sits directly below the entity capture that still has the bug. A test
asserting "N faces in, N delete versions out" makes the two paths fail together
rather than one silently lagging the other.

## Shape

Seed an entity with three faces, delete it, assert three `VersionOpDelete`
captures with distinct pointers. Same for rename. A backend-independent test
against a recording `VersionRecorder`, so it runs on every build tag.
