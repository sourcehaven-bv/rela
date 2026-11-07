---
id: AM-flowcfg-rejects-type-alias
type: automated-measure
title: 'Unit test: flowcfg refuses an entity type alias where a flow names a type'
description: 'TestParse_Problems and TestParse_AliasSubjectReadsNoHiddenProperty load flows.yaml against a test schema with aliases and require subject and create.type to refuse an alias, so the visible: check cannot be bypassed by naming a type through an alias (BUG-GSCDO88).'
kind: test
location: internal/flowcfg/load_test.go
status: active
---
