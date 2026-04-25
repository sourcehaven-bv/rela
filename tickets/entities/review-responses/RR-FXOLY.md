---
id: RR-FXOLY
type: review-response
title: 'Missing test: both edit.form and edit.label empty simultaneously'
finding: 'validate_test.go''s table covers each negative independently but not Edit: &DocumentEdit{} (both fields empty). The validator''s two if-branches are independent (correct: both should fire), but no test pins that contract. Add a row asserting both error substrings appear.'
severity: significant
resolution: 'Added TestValidateConfig_DocumentsEditBothEmpty in validate_test.go: pins the contract that Edit: &DocumentEdit{} produces both error substrings.'
status: addressed
---
