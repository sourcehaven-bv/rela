---
id: metamodel-validation-gaps-test
type: automated-measure
title: 'Test: metamodel rejects fractional integers and empty relation from/to'
description: 'Regression for BUG-70YIIU: asserts an integer property and ParseIntegerValue reject a fractional float (3.5) while accepting integral floats (3.0/7.0), and validateRelationReferences rejects a relation with empty from: or to: while allowing fully-populated relations. Fails if integer validation reverts to accepting any float64 or the relation check stops requiring non-empty from/to.'
kind: test
location: internal/metamodel/validation_gaps_test.go (TestValidatePropertyValue_IntegerRejectsFractional, TestParseIntegerValue_RejectsFractional, TestValidateRelationReferences_RejectsEmptyFromTo, TestValidateRelationReferences_AllowsPopulatedFromTo)
status: active
---
