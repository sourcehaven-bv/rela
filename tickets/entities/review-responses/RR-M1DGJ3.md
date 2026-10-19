---
id: RR-M1DGJ3
type: review-response
title: The -P newline case named in the design review is not tested
finding: "Design review section 13 says TestChannelCLI_* has comma and newline cases for -P. channel_test.go:166 tests only a comma in -P. A newline is tested only through --props-json (channel_test.go:172).\tA future change to -P parsing, such as a different kong separator or trimming, could break multi-line -P values without a failing test.\tAdd `-P $'body=line1\\nline2'` to TestChannelCLI_Loop and assert the stored value and the two `  | ` lines in the table output."
severity: nit
resolution: TestChannelCLI_Loop edits with -P $'body=changed\nline2' and asserts the stored value and both '  | ' lines.
reason: TestChannelCLI_Loop edits with -P $'body=changed\nline2' and asserts the stored value and both '  | ' lines.
status: addressed
---
