---
id: RR-CAPGRD
type: review-response
title: 'Capability guard test only grepped runtime.go, and carried two dead struct fields'
finding: |-
    TestCapabilityRegistrationStaysGated read a single file. The functions it guards are defined in ai.go and http.go, so a registerHTTPModule() call added from ai.go, context.go, or a new mode file would have been invisible to it — precisely the 'second registration site' the guard exists to catch.

    Its table also declared wantOnce and absent fields that were never read: dead configuration in a security guard is worse than none, since it reads as coverage that does not exist.
resolution: |
    The guard now walks every non-test .go file in the package and concatenates them before matching, and the dead fields are gone. Mutation-verified by adding a second registerHTTPModule() call from ai.go (a DIFFERENT file from the one previously grepped): the guard fails with 'found 2'.
severity: minor
status: addressed
---
