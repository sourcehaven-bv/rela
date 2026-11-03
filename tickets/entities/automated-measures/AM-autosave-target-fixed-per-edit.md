---
id: AM-autosave-target-fixed-per-edit
type: automated-measure
title: Autosave writes only to the entity the edit was made on
description: Mounts the real DynamicForm, edits X, switches entityId to Y the way the router does, and asserts no PATCH reaches Y and the form reloads Y. A useAutoSave unit test asserts a 412 refetch or snapshot naming another entity fails closed.
kind: test
location: frontend/src/components/forms/DynamicForm.entitySwitch.test.ts; useAutoSave target-mismatch unit test
status: active
---
