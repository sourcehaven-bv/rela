---
id: AM-lowercase-id-prefix-mints
type: automated-measure
title: 'Test: a lower-case sequential id_prefix mints a valid id'
description: Create entities of sequential types whose id_prefix is lower or mixed case and expect generated ids that pass prefix validation (BUG-4OPP525).
kind: test
location: internal/entitymanager/idprefix_case_test.go (TestCreate_MintsForNonUpperCasePrefix)
status: active
---
