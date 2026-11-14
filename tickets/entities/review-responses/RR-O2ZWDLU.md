---
id: RR-O2ZWDLU
type: review-response
title: Quitting does not save the window geometry
finding: Quit runs the OnShutdown tasks and closes windows without firing WindowClosing, so the frame was only saved on ⌘W.
severity: significant
resolution: OnShutdown saves the frame of the live main window before the app closes it.
status: addressed
---
