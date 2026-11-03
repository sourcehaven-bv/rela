---
id: RR-M8ZHYX
type: review-response
title: Release success path untested
finding: The released line, the --release JSON and the routing to ReleaseAction were never run.
severity: significant
resolution: TestChannelCLI_ActionOutput runs both paths against a fake service and asserts text, JSON and which method was called.
status: addressed
---
