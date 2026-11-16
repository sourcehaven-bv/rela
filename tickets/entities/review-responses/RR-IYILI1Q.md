---
id: RR-IYILI1Q
type: review-response
title: Window geometry save races the Wails destroy listener
finding: Saving the frame in an OnWindowEvent WindowClosing listener runs in a goroutine; Wails' own listener can destroy the window first, so the frame read can fail or read a released window.
severity: significant
resolution: Save now runs in a RegisterHook for WindowClosing, which Wails runs synchronously before the listeners.
status: addressed
---
