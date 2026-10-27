---
id: RR-SKDBYX
type: review-response
title: The AC 8 real-wiring test is missing
finding: 'The plan''s test table lists TestInbox_HiddenRowsChangeNothing in "realtime (fakes) and dataentry (real ACL)". Only the fake version exists (internal/realtime/inbox_test.go:413), and there the HolderFinder fake does the gating, so it cannot catch a wiring regression. One example: scriptReads handing realtime the unrestricted lookup under a policy, or holderFinder dropping FaceIn. internal/dataentry/inbox_handler_test.go has no `visible:`-hidden holder at all. Fix: add a dataentry test that uses the real acl.Declarative and a FieldRedactor hiding `owner` on some tickets, and compare the JSON of a project with and without the hidden rows (count, truncated, next_cursor), as AC 8 states.'
severity: minor
resolution: 'Added TestInbox_HiddenRowsChangeNothingRealACL with the real declarative ACL and a visible: grant hiding owner'
status: addressed
---
