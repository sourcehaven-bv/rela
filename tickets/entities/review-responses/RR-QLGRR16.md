---
id: RR-QLGRR16
type: review-response
title: "Toasts and editor fullscreen draw over the session screen"
finding: "The session screen sits at 1201, below toasts (1300) and editor fullscreen (10000); the comment claimed it was above everything."
severity: minor
resolution: "Addressed in the comment; the order is kept. WorkspaceAccessScreen.vue is changed in another PR (finding 5), and error toasts are now suppressed under it."
status: addressed
---
