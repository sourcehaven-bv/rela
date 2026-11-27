---
id: RR-L9KDNV6
type: review-response
title: startOutputs coalesces calls onto the first ctx and WaitGroup
finding: A rerun uses the ctx and rounds of the call that started the goroutine; later calls' arguments are ignored. Safe only because every call comes from one lead.
severity: minor
resolution: The startOutputs godoc now states that every call within one lead passes the same ctx and rounds.
status: addressed
---
