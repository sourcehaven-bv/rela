---
id: RR-G2P84C
type: review-response
title: 'State names: realtimecfg and the store disagree on reserved names'
finding: channels.ValidName refuses Windows device names (con, aux, nul, lpt1, ...) as well as failing namePattern. realtimecfg checks only namePattern. If PrepareSessionStep validates state names with ValidName, a config with a state named aux loads, but every step into it fails with ErrInvalidMessage (422 for users, permanent for triggers). State names are never path segments, so validate them in the store with the plain name grammar, not ValidName, or refuse the same set at load. Add a test row for it.
severity: minor
resolution: 'Plan updated: the store checks state names with ValidStateName matching realtimecfg; a test moves a session into a state named aux (AC 3).'
status: addressed
---
