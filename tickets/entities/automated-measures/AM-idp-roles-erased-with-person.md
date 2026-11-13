---
id: AM-idp-roles-erased-with-person
type: automated-measure
title: Stored IdP roles are erased when their person is deleted or renamed
description: TestIDPRoles_ErasedWithThePerson deletes and renames a person through the entity manager and asserts no IdP role record stays bound to it.
kind: test
location: internal/appbuild/idproles_erase_test.go
status: active
---

Fails when the idproles store is no longer subscribed to the entity manager's
delete and rename hook.
