---
id: AM-rule-refusal-names-no-entity
type: automated-measure
title: Relation rule refusal names no entity
description: A PATCH whose added edge the far end's write rule refuses must answer 422 without naming the target or any other entity id in the problem.
kind: test
location: internal/dataentry/relation_rule_test.go (TestRelationRule_PatchJudgesTheStateItLeaves)
status: active
---
