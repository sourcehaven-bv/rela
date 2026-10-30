---
id: RR-IHD9E9
type: review-response
title: Warning wording is ungrammatical and does not match the docs
finding: 'internal/realtimecfg/message_in.go:87 says "drop the in: that only repeat on: keys". The subject is plural but "in:" reads as singular, and the guide (GUIDE-realtime.md, load-check bullet) says "drop the `in:` lists that only repeat the `on:` keys". Fix: change the text to "drop the in: lists that only repeat the on: keys, or drop default_in". Update the constant in TestDefaultIn_Checks (default_in_test.go:295-296) and the plan''s AC 9 text.'
severity: minor
resolution: 'Changed the warning to "drop the in: lists that only repeat the on: keys, or drop default_in"; the shared constant noEffectWarning in default_in_test.go and the AC 9 text in plan-s5oj7s-body.md match it.'
status: addressed
---
