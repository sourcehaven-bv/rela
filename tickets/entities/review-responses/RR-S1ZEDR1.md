---
id: RR-S1ZEDR1
type: review-response
title: Window shrinks 1pt on every close and reopen
finding: Wails on macOS creates the window 1pt smaller than asked, while Size reads the frame, so each save and restore lost 1pt. Found in manual testing.
severity: minor
resolution: mainWindowOptions adds restoreInset (1 on macOS) to a saved size. Pinned in TestMainWindowOptions.
status: addressed
---
