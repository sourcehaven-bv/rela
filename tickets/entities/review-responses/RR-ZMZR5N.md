---
id: RR-ZMZR5N
type: review-response
title: Reopen rule uses only the last reopen and stretches a restarted deadline on a second close/reopen
finding: 'timerDue (plan Approach 4) restarts when ReopenedAt > base and the ORIGINAL due <= ReopenedAt. Only the last reopen is stored, so the restart point is lost. Example, P=72h from session at T0: close T0+10h, reopen T0+100h, due becomes T0+172h. Close at T0+110h, reopen at T0+120h (before 172h): base+P = T0+72h <= T0+120h, so the formula yields T0+192h. The plan''s own stated semantics (and the user decision) say a timer that had not come due keeps its deadline, so the legal deadline stretches by 20h. The claim ''Only the last reopen is stored, which is enough'' (plan line 513) is false. Plan change: persist the restart anchor when a timer actually came due while closed (for example a session_resumed_at written by the reschedule the round already does after a reopen, under the CAS, and used as max-anchor only when base+P <= resumed_at), or explicitly redefine the rule and document it. Add a TestTimerDue row and a ClosedFreezes subcase for two closures.'
severity: significant
resolution: 'Plan updated: per-timer restart anchors (session_restarts) written in the reopen/restore step via ResumeSchedule replace the last-reopen rule; AC 10 and AC 11 test two close/reopen cycles keeping T0+172h.'
status: addressed
---
