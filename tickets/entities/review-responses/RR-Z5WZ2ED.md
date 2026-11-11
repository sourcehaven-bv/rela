---
id: RR-Z5WZ2ED
type: review-response
title: Clear on sign-out is not implementable
finding: Sign-out is a full-page href; the SPA runs no code then.
severity: significant
resolution: Key prefs and seen set by relaBase() + me.user; notifier starts only after /_me answers and skips the unknown user.
status: addressed
---
