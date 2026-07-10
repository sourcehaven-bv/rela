---
id: AM-acl-write-subject-type-invariant
type: automated-measure
title: 'Integration test: every entity-write path authorizes against the stored type'
description: For every entity-write entry point (v1 PATCH, sync PUT, ...), asserts the ACL subject type equals the stored entity's type and a body/claimed type differing from the stored type is rejected on update. Catches any future body-driven-subject path (the class that produced BUG-ZWTDH9). P4 invariant test.
kind: test
location: internal/dataentry / internal/entitymanager (integration test, added in the BUG-ZWTDH9 fix PR)
status: proposed
---
