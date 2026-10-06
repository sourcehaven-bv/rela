---
id: AM-edit-form-reads-in-page-world
type: automated-measure
title: The edit form reads an entity in the world the page showed
description: An e2e spec opens a control's edit form from the editorial world and asserts that the form shows a relation to a draft-only policy, which only that world serves, keeps it when another link is added, and can remove it. A second spec switches world in the open form and asserts the relations reload. Unit tests pin editFormRoute carrying the world and DynamicForm fetching in the route's world.
kind: test
location: e2e/tests/faces-edit-relations.spec.ts
status: active
---

Guards against an edit entry point or the form dropping `?world=`. The e2e
spec fails on the code before the fix at the picker tile assertion.
