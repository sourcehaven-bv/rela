---
id: TKT-0POF9KR
type: ticket
title: Entity reference field type on record types
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split from TKT-EV8U8T4. An `entity` field type on record types, with `target:`
naming the entity type, for dimensions such as the person who booked hours.

Decide what a reader sees when the referenced entity is hidden from them, and
validate the reference on append.

## Acceptance

- An append with an `entity` field naming a missing or wrong-type entity is
refused.
- A reader who cannot read the referenced entity does not learn whether it
exists.
