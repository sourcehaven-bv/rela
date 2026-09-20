---
id: AM-acl-subject-names-its-face
type: automated-measure
title: 'Guard test: every faceless acl.EntitySubject is argued for'
description: |-
    Since BUG-Y0GNSB P5 the primary enforcement is the TYPE, not a scan:
    acl.EntitySubject has unexported fields and is built only by
    acl.NewEntitySubject(typ, id, face) or acl.NewFacelessEntitySubject(typ,
    id). The literal that caused the bug - acl.EntitySubject{Type, ID}, which
    compiled fine and silently authorized the DEFAULT face while the store
    wrote the PUBLISHED one - no longer compiles anywhere.

    This test guards the residual discretionary choice the compiler cannot
    make: which of the two constructors a caller picks. NewFacelessEntitySubject
    is a one-word way back to the old behavior, so the test repo-scans every
    non-test source and fails on any call from a file not in its
    `facelessAllowed` allowlist, each entry carrying a written reason. It is an
    ALLOWLIST rather than an exemption list, so a new faceless call site fails
    closed until someone argues for it, and it also fails if an allowlist entry
    names a file that no longer exists.

    Two permitted sites today: internal/entitymanager/manager.go (rename
    re-keys the whole entity family, so naming one face would assert a
    narrower scope than the operation has) and internal/docs/assert_acl.go
    (the allows{}/refuses{} Lua surface has no `face` field).

    Replaces TestEveryEntitySubjectNamesItsFace, which scanned
    internal/entitymanager for literals omitting Face. That test became
    vacuous under P5 - it searched for a construct the language now rejects,
    so it could only ever pass.

    Paired with TestFacedIDWrite_* in internal/dataentry, which pins the
    end-to-end behavior no scan can see (a call that passes the WRONG face
    expression). See also BUG-HC6I2T: the old scan was satisfied by a literal
    that set Face to a value the write did not use.
kind: test
location: 'internal/acl/facelessguard_test.go (TestFacelessSubjectsAreArguedFor)'
status: active
---
