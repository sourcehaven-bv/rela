---
id: AM-autosave-two-tabs-guarantees
type: automated-measure
title: 'Property test: two tabs autosaving one entity keep the autosave guarantees'
description: Two useAutoSave instances against the fake API with failing, empty-412 and lost-answer faults; no blind overwrite, no silent loss, no false conflict, and a held field always shows its error.
kind: test
location: frontend/src/composables/useAutoSave.twoClients.prop.test.ts
status: proposed
---
