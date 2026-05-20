---
id: data-entry-form-relation-side-checks
type: automated-measure
title: Validator checks for inverse-name and wrong-side form relations
description: 'Table-driven tests in `internal/dataentryconfig/validate_test.go` pin every branch of `validateForms`'' relation handling. `inverseTestMetamodel` parses a metamodel through the real loader so `inverseOwners` is populated the same way the runtime sees it. Branches covered: inverse-name produces an error pointing at the canonical name + `direction: incoming`; wrong-side binding with no direction produces a hint to flip the direction; wrong-side with explicit direction errors without the hint; `direction: incoming` on the source side also errors; correct-shape configs (canonical+outgoing, canonical+incoming) and multi-source-type relations pass.'
kind: test
location: internal/dataentryconfig/validate_test.go (TestValidateConfig_FormRelation*)
status: active
---
