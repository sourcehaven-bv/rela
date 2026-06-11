---
id: MEAS-create-field-affordance-test
type: automated-measure
title: v1 create-path field-affordance enforcement test
description: Asserts POST /api/v1/<type> rejects writing hidden/read-only fields and filtered enum options at create time, with the same 403 + rule_id shape as the PATCH path. Prevents regression of BUG-Q60V.
kind: test
location: internal/dataentry/api_v1_test.go (TestHandleV1CreateEntity_FieldAffordances)
status: active
---
