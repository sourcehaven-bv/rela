---
id: RR-Z3M5OFQ
type: review-response
title: Exports used only by tests
finding: triggers.Declares, triggers.NewEngine and projectsetup.ConvertAutomationsYAML have no production callers outside tests.
severity: nit
resolution: 'Deleted the unused triggers.Declares and its test; unexported projectsetup.ConvertAutomationsYAML. triggers.NewEngine stays exported: entitymanager tests in another package use it and it was exported before the move.'
status: addressed
---
