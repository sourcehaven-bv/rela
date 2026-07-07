---
id: incoming-picker-create-persist-test
type: automated-measure
title: 'Test: incoming-direction relation picker persists selections on the create form'
description: Unit (RelationPicker.test.ts) + e2e (reverse-relations.spec.ts) coverage asserting that an incoming-direction relation picker persists selections made on the create form, including untouched-picker-emits-nothing and single-select cases. Both verified to fail before the BUG-10IPBP fix.
kind: test
location: frontend/src/components/forms/RelationPicker.test.ts (unit), e2e/tests/reverse-relations.spec.ts (e2e, BUG-10IPBP)
status: active
---

Locks in the fix for BUG-10IPBP.

- **Unit** (`RelationPicker.test.ts`, describe "incoming direction on create"): mounting an incoming picker with no `entityId` and selecting a peer must emit `incoming-changed` with the peer as a pure addition; removing it emits an empty desired set. Verified to fail before the fix.
- **E2E** (`reverse-relations.spec.ts`, "non-cards picker add persists on CREATE, not just edit"): create a new feature, pick TASK-002 in its incoming "Implemented by" picker, submit, and assert `TASK-002 --implements--> <new feature>` exists (read from the source side). Verified to fail before the fix.
