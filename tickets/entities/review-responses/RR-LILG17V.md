---
id: RR-LILG17V
type: review-response
title: Configure could redefine a captured card
finding: '[security] A capture task names a card by title in data-entry.yaml, which the Configure space can edit (title, display, property, list). An editor could point a captured card at data only run_as reads and read the stored figure on the owner.'
severity: significant
resolution: configedit.captureProblems refuses a save that changes any card with the captured title, its list, or the schema property and query scopes it reads; an unparseable schedules.yaml refuses the save. Tests in internal/configedit/captures_test.go; documented in the data-entry and scheduled-tasks guides and .claude/rules/configedit.md.
status: addressed
---
