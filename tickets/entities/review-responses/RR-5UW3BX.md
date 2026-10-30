---
id: RR-5UW3BX
type: review-response
title: The equivalence table cannot run over both fixtures as written, and the plan contradicts AC 4
finding: 'AC 6 runs the TestAffordances_MessageIn table over pocRealtime and over pocRealtimeOffer. AC 4 says the existing tests stay green "without edits". The table is inline in the test function, so reusing it means extracting it, which is an edit. Worse, the stranded row''s setup is fixed to pocRealtime("") and rewrites its in: strings (internal/realtime/session_affordance_test.go:621-629). Over the offer fixture, that row would swap in the config without offer and pass for the wrong reason. Change: (1) Reword AC 4 to "existing expectations unchanged". (2) Extract the rows into a function that takes the fixture text, and make setup closures build the stranded config from that text. (3) Assert in the stranded row that the snapshot installed has offer set. (4) Move AC 5 (TestOffer_PoCEquivalent) to internal/realtimecfg. That package already has pocRealtime(in, session) and pocIn (message_in_test.go:74-104). The test is then pocRealtime(pocIn minus the 17 keyed types, "      offer: transitions\n") against pocRealtime(pocIn, ""), comparing In for every type. A string-munged copy of the fixture in internal/realtime is not needed.'
severity: significant
resolution: AC 4 now says existing expectations are unchanged, and AC 6 moves the rows into messageInRows(fixture) whose harness and setup closures build from that text, with the stranded row asserting its snapshot has default_in set. AC 5 moved to internal/realtimecfg as TestDefaultIn_PoCEquivalent over pocRealtime(in, session) and pocIn.
status: addressed
---
