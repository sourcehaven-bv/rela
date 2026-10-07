---
id: AM-history-undeclared-props-redacted
type: automated-measure
title: History redaction denies snapshot properties the schema no longer declares
description: A history read test seeds a snapshot carrying a property later removed from the schema and its visible block, and asserts the property is absent for a reader without an affirmative grant, on the HTTP API and in rela.get_version.
kind: test
location: internal/dataentry/history_handler_test.go
status: proposed
---
