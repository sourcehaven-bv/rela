---
id: RR-YDMHZT
type: review-response
title: Bus lifecycle and the feed field race with handlers
finding: Assigning and nilling a.feed in Start/StopWatching races with request goroutines; nil feed meant a second wire protocol; Run/Close order was unspecified.
severity: significant
resolution: Bus built in NewApp and never reassigned; Start(src) rejects nil, a second Start and Start after Close; Close is idempotent, safe before Start, and waits for the pump. One wire protocol even when never started (desktop).
status: addressed
---
