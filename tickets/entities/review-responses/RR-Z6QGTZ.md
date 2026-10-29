---
id: RR-Z6QGTZ
type: review-response
title: No test for the hidden-on-error branch, and the Guards load table cannot tell shadowing from undecided
finding: 'internal/realtimecfg/buttons.go:272-275: the branch where an earlier guard errors for the preset and admits no class of the entry (recorded as undecided) has no test. The only error case, ''earlier guard errors'' at buttons_test.go:328, uses an entry with no roles, so it takes the refusal branch. In addition, TestButtons_Guards (buttons_test.go:312) compares only hiddenBy(), but exclusive() (buttons.go:398) reads b.shadowing. An entry that is misfiled as undecided instead of shadowing would pass this table and then wrongly refuse duplicate labels. Fix: add a load case ''earlier guard errors for a disjoint role'' (roles: [lead], regex error; button entry roles: [member]) that expects it to load with hidden [0], plus an Offers check that lead+member is not offered the button. Give the load table separate want fields for shadowing and hidden.'
severity: minor
resolution: Added load cases for an earlier guard that holds for a disjoint role and one that errors for a disjoint role (hidden [0], loads), plus TestButtons_ErroringDisjointGuardHides checking member is offered the button and lead+member is not. The load table now has separate shadowing and hidden want fields compared with slices.Equal.
status: addressed
---
