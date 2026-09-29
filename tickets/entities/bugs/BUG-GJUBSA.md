---
id: BUG-GJUBSA
type: bug
title: Rename affordance and doc ACL assertions decide rename and delete per face
description: The data-entry rename affordance and the doc ACL assertions decide rename and delete on one face while the manager authorizes every face.
priority: medium
effort: s
status: backlog
---

## Problem

Rename and family delete authorize every face of the family (#1708, #1714). Two
surfaces still decide them per face:

- The data-entry rename affordance (`internal/dataentry/affordances.go`) offers rename when the SERVED face allows `update`. A principal with a draft-only grant sees the action, and the server refuses it.
- The documentation ACL assertions (`internal/docs/assert_acl.go`) evaluate rename and delete against the zero face, so a documented claim about a faced type can pass or fail for the wrong reason.

## Expected

Both evaluate the same family-wide rule the manager enforces
(`authorizeFamily`).
