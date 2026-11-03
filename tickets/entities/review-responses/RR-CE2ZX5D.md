---
id: RR-CE2ZX5D
type: review-response
title: EntityDetail body editor chose its save target at emit time
finding: onBodyInput called editAutoSave() when the editor emitted. EntityBody was not keyed per row, so an editor opened on A survived a route change to B and its next emit saved A's whole body to B with B's own content token, so no 412 stopped it.
severity: critical
resolution: 'EntityBody now emits edit; EntityDetail pins the instance at edit start (bodySave) and mirrors into the view only when that row is on screen. EntityBody is keyed by servedRef and flushes its editor on unmount, so a switch closes it and its last keystrokes go to the row it was opened on. Test: ''writes a body edit to the row the editor was opened on after a switch''.'
status: addressed
---
