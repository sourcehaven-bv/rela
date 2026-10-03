---
id: RR-D3DZPU
type: review-response
title: 'Listen goroutine can block on unbuffered sends after Shutdown'
finding: 'neoq postgres_backend.go listen(): `c <- notification` and `p.listenConnDown <- true` block forever once workers or listenerManager have exited.'
severity: significant
resolution: 'Both sends now select on ctx.Done() and return.'
status: addressed
---
