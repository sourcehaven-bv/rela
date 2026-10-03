---
id: RR-48NUVN
type: review-response
title: e2e refusal assertion matches error wording
finding: expectActionRefused asserts the dialog contains 'forbidden', which ties the test to the message text.
severity: minor
reason: The script-error dialog exposes no stable code or test id. The word is the ACL error kind, which is stable. A test id is a frontend change outside this PR.
status: wont-fix
---
