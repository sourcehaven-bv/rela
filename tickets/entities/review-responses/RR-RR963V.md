---
id: RR-RR963V
type: review-response
title: App.AttachRender exceeds the plimsoll cap
finding: App is pinned at max-methods=92 and already has 92.
severity: significant
resolution: 'Plan: separate attachRenderer type built by a constructor that rejects nil collaborators; gate cores as free functions or methods on exportHandler/attachmentHandler.'
status: addressed
---
