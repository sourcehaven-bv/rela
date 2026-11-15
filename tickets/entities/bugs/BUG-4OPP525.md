---
id: BUG-4OPP525
type: bug
title: Lower-case sequential id_prefix can never mint a valid id
description: A sequential type with a lower-case id_prefix mints an upper-case id that the case-sensitive prefix check refuses.
priority: low
effort: s
status: backlog
severity: minor
---

## Description

A sequential type whose `id_prefix` is lower case, such as `nt-`, can never
create an entity with a generated id. `entity.SequentialID` upper-cases the
prefix and mints `NT-001`, and `metamodel.Validator.ValidateEntity` then
compares the id with the declared prefix case-sensitively and refuses it:
"entity ID NT-001 does not match any prefix for type note: [nt-]".

Found while adding id minting edge-case tests for BUG-RLBXTG5. It predates
TKT-FED6FMM: `GenerateSequentialID` upper-cased the prefix too.

## Expected

Either the metamodel rejects a lower-case sequential `id_prefix` at load time,
or minting keeps the declared case, or the prefix check folds case like the rest
of id identity does.
