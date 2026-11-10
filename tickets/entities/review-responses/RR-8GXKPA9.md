---
id: RR-8GXKPA9
type: review-response
title: A rename lifts the outside_edits ceiling
finding: Runs are keyed by subject id, so renaming the subject left its runs under the old id and the new id had no rules.
severity: significant
resolution: A flowRunAliases subscriber on the alias fanout rekeys runs on rename. TestOutsideEdits_RenameKeepsRules pins it, and fails without the subscriber. GUIDE-flows says a rename moves runs.
status: addressed
---
