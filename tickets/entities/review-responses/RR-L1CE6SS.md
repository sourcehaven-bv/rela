---
id: RR-L1CE6SS
type: review-response
title: A closing main window still counts as live
finding: Between WindowClosing and Wails removing the window, GetByName and GetAll still return it, so a Dock click or Open Recent in that gap reused a window about to be destroyed.
severity: minor
resolution: mainWindows records the closing window; live() and openWindowCount skip it. Covered by TestOpenWindowCount.
status: addressed
---
