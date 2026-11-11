---
id: RR-N932P8K
type: review-response
title: Constructor failure warns on every read
finding: Chrome on Android grants permission but throws in new Notification, so settings shows it working and each read warns.
severity: minor
resolution: The first failure calls markUnsupported (sticky until reload), which stops the notifier and makes Settings say unsupported. Test added.
status: addressed
---
