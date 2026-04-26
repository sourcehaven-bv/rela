---
id: RR-MG0LG
type: review-response
title: Data entry web UI silently drops ScriptErrors and LoadErrors
finding: 'The dataentry web app runs validations through validator.GenericValidator.CheckRule, which returns only entity IDs. ScriptErrors and LoadErrors from validation.Result are discarded. The plan never noticed this surface — UX improvement is invisible in the browser. Broken Lua rules vanish silently for browser users. Location: internal/dataentry/analyze.go:399-432, internal/validator/validator.go:61-73.'
severity: significant
resolution: Added Validator.CheckRuleFull which returns RuleResult{Violations, ScriptErrors, LoadErrors}. GenericValidator implements both methods. dataentry/analyze.go now calls CheckRuleFull and emits AnalysisIssues for ScriptErrors and LoadErrors so broken Lua rules are visible in the browser surface instead of silently dropped. Two new dataentry tests cover both error paths. Commit f0b5684.
status: addressed
---
