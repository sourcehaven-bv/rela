---
id: RR-7NY5AZ
type: review-response
title: Warning output untested
finding: warningText and its Path/Detail branches were never run.
severity: significant
resolution: TestChannelCLI_ActionOutput feeds two warnings, one with a control character, and asserts the exact lines.
status: addressed
---
