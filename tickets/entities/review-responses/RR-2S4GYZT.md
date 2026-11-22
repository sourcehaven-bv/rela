---
id: RR-2S4GYZT
type: review-response
title: "A 401 replaced the app and lost unsaved input"
finding: "The ended-session page replaced the whole app, so a half-filled form and its leave guard were gone."
severity: critical
resolution: "Addressed: an ended session is a dialog over the app; the page underneath stays mounted. Pinned by WorkspaceAccessScreen.test.ts."
status: addressed
---
