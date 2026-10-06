---
id: AM-section-create-prelink-direction
type: automated-measure
title: Section create links the new entity to the page entity in the relation's direction
description: 'E2E test: a section create on an entity whose type has a dashed id_prefix, opening a form with no field for the relation, creates the entity and writes page --relation--> new. Unit tests pin entityTypeForId over both prefix spellings and the inverse-key payload.'
kind: test
location: e2e/tests/create-prelink.spec.ts
status: proposed
---
