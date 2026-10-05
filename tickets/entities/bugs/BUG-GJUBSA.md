---
id: BUG-GJUBSA
type: bug
title: Rename affordance and doc ACL assertions decide rename and delete per face
description: The data-entry rename affordance and the doc ACL assertions decide rename and delete on one face while the manager authorizes every face.
priority: medium
effort: s
why1: computeActions asked the ACL about rename on the served face only and assert_acl evaluated rename and delete on the zero face.
why2: 'Both were written when rename and delete were per-row operations; #1708 and #1714 made them family-wide in the manager only.'
why3: The family rule lived in entitymanager.authorizeFamily with no shared predicate the affordance and the docs checker could call.
why4: Affordance contract tests compare _actions with the write per face and had no case where the faces disagree.
why5: A change to an authorization rule has no checklist of the surfaces that predict it (affordances, docs assertions, aclaudit).
prevention: TestFaceGrant_RenameAffordanceIsFamilyWide and claimfaces_test.go pin the family rule on both surfaces; AM-family-ops-affordance-matches-manager tracks the measure.
status: done
---

## Problem

Rename and family delete authorize every face of the family (#1708, #1714). Two
surfaces still decide them per face:

- The data-entry rename affordance (`internal/dataentry/affordances.go`) offers rename when the SERVED face allows `update`. A principal with a draft-only grant sees the action, and the server refuses it.
- The documentation ACL assertions (`internal/docs/assert_acl.go`) evaluate rename and delete against the zero face, so a documented claim about a faced type can pass or fail for the wrong reason.

## Expected

Both evaluate the same family-wide rule the manager enforces
(`authorizeFamily`).
