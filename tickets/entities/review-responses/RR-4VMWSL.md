---
id: RR-4VMWSL
type: review-response
title: TestParse_MessageInBrokenType cannot fail for the reason it names
finding: 'internal/realtimecfg/message_in_test.go:266 gives progress a valid in: [planning, working] next to a bad post role, then asserts no in: problem. With a valid in:, messageInCheck.one adds no problem whether or not the mt == nil skip at message_in.go:40 runs, so the test passes even if that guard is deleted. Its purpose is to prove that a type that failed to compile is skipped and does not panic. Fix: make the in: invalid too (for example [nope] or ~) and assert that only the post[0] problem is reported. Also make sure the chosen breakage actually leaves ct.MessageTypes[mn] nil (validate.go:180 stores the type only when messageType returns non-nil). Otherwise pick a breakage that does.'
severity: minor
resolution: 'TestParse_MessageInBrokenType now pairs a bad post role with an invalid in: and asserts both problems; the new TestMessageInCheck_SkipsMissingType calls the check with a type the compiled channel type lacks, and it panics when the guard is removed (checked).'
status: addressed
---
