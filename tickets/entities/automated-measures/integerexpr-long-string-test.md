---
id: integerexpr-long-string-test
type: automated-measure
title: 'Test: the Postgres integer reading never fails on a long digit string and agrees with ParseInt'
description: Guards against BUG-T6LRSCE. TestIntegerExprAgreesWithGo stores strings of 200000 digits, leading zeros and signed boundary values through jsonb and compares the SQL result with propmatch.Integer.
kind: test
location: internal/store/pgstore/integerexpr_test.go:TestIntegerExprAgreesWithGo
status: active
---
