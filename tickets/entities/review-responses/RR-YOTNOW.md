---
id: RR-YOTNOW
type: review-response
title: Process-level state is created per App
finding: Each NewApp builds its own upload locker and limiter; exports build their own transform engine (export.go:161); the webhook seenSet is per App (webhook.go:79). A rebuild would split locks, double limits and reset replay protection.
severity: significant
resolution: 'Plan changed: these are built once in main and injected into every App.'
status: addressed
---
