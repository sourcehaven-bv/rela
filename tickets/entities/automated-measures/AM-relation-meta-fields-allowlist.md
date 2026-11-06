---
id: AM-relation-meta-fields-allowlist
type: automated-measure
title: 'Relation meta fields: grants are an allowlist'
description: A compiled policy whose role names one relation meta field refuses every other meta key on PATCH and conflict resolve; a role without a meta list that may update the source lifts it.
kind: test
location: internal/affordances/relation_meta_write_test.go; internal/dataentry/conflict_fields_acl_test.go
status: active
---
