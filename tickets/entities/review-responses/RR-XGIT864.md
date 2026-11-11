---
id: RR-XGIT864
type: review-response
title: 'Design: a lost compare-and-set is normal between sibling branches'
finding: The engine treats ErrConflict as another engine took over. With branches a sibling's write causes conflicts, which would leave pending marks unsettled and report ProblemMoved for a branch that did not move.
severity: significant
resolution: 'Plan updated: every engine write goes through one bounded read-modify-write helper; on conflict it re-reads and re-applies the change if its own token is still at the same generation and mark, and only reports lost when the token itself moved or is gone.'
status: addressed
---
