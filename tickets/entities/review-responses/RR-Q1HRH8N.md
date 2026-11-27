---
id: RR-Q1HRH8N
type: review-response
title: The hotkey callback runs on its own goroutine
finding: The hotkey callback runs on its own goroutine
severity: significant
resolution: The callback takes one snapshot of the active app under the read lock; window work runs through InvokeAsync; the popup pointer is guarded by a mutex so two presses open one window.
status: addressed
---
