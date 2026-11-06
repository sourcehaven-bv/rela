---
id: RR-8CLWWUB
type: review-response
title: Unset session nil versus empty reminders untested
finding: flow.Read now carries stored reminder records of an unset instance; nil (keep) and empty (store none) must stay distinct.
severity: significant
resolution: Added TestStepSchedule_UnsetKeepsNilVersusEmpty.
status: addressed
---
