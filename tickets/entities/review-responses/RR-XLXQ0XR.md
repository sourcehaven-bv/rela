---
id: RR-XLXQ0XR
type: review-response
title: The push read past the read gate while the pull now honours it
finding: After merging develop (TKT-A6U657Q) the pull skips and never names local rows the principal cannot read. The push still listed raw rows, sent them and named them in its report, so a principal could push rows and fields it cannot read.
severity: significant
resolution: 'The push reads each local row through Deps.Gate: an unreadable row is not pushed and not named, a field withheld by visible: is not sent, and share is judged on the gated row. TestPush_SendsOnlyWhatThePusherReads; docs updated.'
status: addressed
---
