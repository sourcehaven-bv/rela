---
id: RR-BRE9WW
type: review-response
title: ButtonOptionsView vs SessionButtonView fields are described two ways
finding: Approach 6 introduces ButtonOptionsView and also says SessionButtonView gains the three fields (internal/realtime/session.go:121-124). Say whether SessionButtonView embeds ButtonOptionsView or repeats the fields; the apiwire side is explicit (written out for the wire golden), the realtime side is not.
severity: nit
resolution: SessionButtonView embeds ButtonOptionsView in internal/realtime, while apiwire writes the fields out for the wire golden.
status: addressed
---
