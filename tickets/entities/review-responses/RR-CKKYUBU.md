---
id: RR-CKKYUBU
type: review-response
title: "Smaller review points"
finding: "overflow-x hidden still allows a scripted scroll; the client-only items_empty sat on the server wire type; the single-language rule had no test; the shell test builds its expected value with bootShell."
severity: minor
resolution: "items_empty moved to a client type; the single-language rule moved into LanguageSetting with a test. overflow-x stays hidden because clip computes to hidden next to a scrolling y axis, and the e2e test checks scrollLeft. bootShell itself is pinned by TestBootShell."
status: addressed
---
