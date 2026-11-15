---
id: AM-lowercase-id-prefix-mints
type: automated-measure
title: 'Test: a lower-case sequential id_prefix mints a valid id'
description: Create an entity of a sequential type whose id_prefix is lower case and expect a generated id that passes prefix validation, or expect the schema to be refused at load (BUG-4OPP525).
kind: test
location: internal/entitymanager (to be added with the fix)
status: proposed
---
