---
id: RR-KX8GVE
type: review-response
title: Compare normalized due times, and make the minimum check calendar-aware
finding: 'The round reschedules when the derived due ''differs from the stored one''. The stored value is microsecond-truncated (micros on SQL, NormTime on file), so a derived value that is not passed through channels.NormTime always differs, and every listed session is rewritten on every round. The 1m floor check must also handle calendar periods: Period.Fixed returns false for months (internal/realtimecfg/period.go:63). Plan change: say that both sides go through NormTime, and that the floor check treats a calendar period as passing. Add a TestTimers_NoRewriteWhenUnchanged budget row (zero SetSession calls).'
severity: nit
resolution: 'Plan updated: derived and stored due times both go through channels.NormTime; the 1m floor applies to fixed periods and calendar periods pass; AC 12 TestTimers_NoRewriteWhenUnchanged pins zero SetSession calls.'
status: addressed
---
