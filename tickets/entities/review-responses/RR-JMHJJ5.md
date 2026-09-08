---
id: RR-JMHJJ5
type: review-response
title: RequiresCurrentUser would fail open on an IR node inspect() does not visit
finding: Program.inspect's type switch had no exhaustiveness guard. A future node type carrying sub-expressions would be skipped, References()/Functions() would omit the variable, RequiresCurrentUser would return false and MatchesAs would skip the binding — surfacing as a generic 'binding not provided' eval error rather than ErrNoCurrentUser, bypassing the next_action_identity_required translation.
severity: minor
resolution: inspect now has a default arm that panics naming the unhandled node type, so adding a node without extending the dependency walk fails the first test that compiles it.
status: addressed
---
