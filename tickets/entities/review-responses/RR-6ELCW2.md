---
id: RR-6ELCW2
type: review-response
title: Kind policy has no datetime case and no trigger-side exactly-once check
finding: internal/realtimecfg/reminders_test.go:221-226 - AC 5 lists datetime, but the table covers integer, boolean, ref and date only. The trigger path's kind refusal is pinned only by load_test.go "trigger prop ref" through assertProblem, which does not fail on a second problem at the same key. Add a datetime row and one trigger case through assertOnlyProblemAt so a regression that also runs NormalizeProps after a policy refusal is caught on both callers.
severity: minor
resolution: "addressed\tTestParse_ReminderLiteralPolicy has a datetime row, and TestParse_TriggerLiteralOrder checks the trigger ref refusal through assertOnlyProblemAt so a second problem at the same key fails the test."
status: addressed
---
