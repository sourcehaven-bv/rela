---
id: RR-WF3MB6
type: review-response
title: Exhausted attempts leave bases moved and conflicts unreported
finding: resolveConflicts moves bases to theirs in attempts 1-2; if attempt 3 also 412s the raw error is thrown and reportConflicts never runs. The next edit overwrites silently. Untested.
severity: significant
resolution: 'Base moves for conflicts are collected in SendResult.conflicts (propBases / contentBase) and applied only in mergeServerResponse after a successful round. A save that runs out of attempts throws with every base unmoved. Test: ''moves no base when the attempts run out'' (mutation-verified).'
status: addressed
---
