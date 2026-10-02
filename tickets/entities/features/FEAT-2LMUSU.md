---
id: FEAT-2LMUSU
type: feature
title: Undo a delete in data-entry
summary: A delete in the data-entry app can be undone for a short while from the confirmation toast.
description: A web delete hides the entity and its relations at once and keeps the rows aside for an undo window (60s by default). The Undo button in the toast restores them unchanged through POST /{plural}/{id}/restore; after the window a background job purges them for good.
priority: medium
status: implemented
---
