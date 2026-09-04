---
id: AM-acl-subject-names-its-face
type: automated-measure
title: 'Guard test: every acl.EntitySubject literal names the face it authorizes'
description: |-
    Source-scans internal/entitymanager and fails on any acl.EntitySubject
    composite literal that does not set Face, with a per-literal
    `facesubject:no-face` opt-out that requires a written reason.

    Catches the class that produced BUG-Y0GNSB - a security-relevant dimension
    added to the authorization subject whose meaningful zero value ("the
    default face") makes every un-updated construction site compile silently
    and assert the wrong resource. A per-file exemption list was rejected as
    too coarse, since manager.go holds six subject literals of which only
    rename is legitimately faceless.

    Paired with TestFacedIDWrite_* in internal/dataentry, which pins the
    end-to-end behaviour the scan cannot see (a literal that sets Face to the
    WRONG expression).
kind: test
location: 'internal/entitymanager/facesubjectguard_test.go (TestEveryEntitySubjectNamesItsFace)'
status: active
---
