---
id: RR-EU92R4
type: review-response
title: ButtonOptions.IsZero is exported production API used only by a test
finding: 'internal/realtimecfg/buttonoptions.go:60: IsZero has one caller, buttonoptions_test.go:180. No marshaller uses it, because ButtonOptions is never encoded. Fix: delete it and compare with ButtonOptions{} in the test, or unexport it.'
severity: nit
resolution: Deleted ButtonOptions.IsZero; the test compares with ButtonOptions{}.
status: addressed
---
