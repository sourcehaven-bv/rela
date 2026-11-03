---
id: RR-QOMK7P
type: review-response
title: 'The load-time when: check duplicates whenHolds and depends on the load date'
finding: 'internal/realtimecfg/validate.go:784-826. triggerMessage re-implements realtime.whenHolds (binding message fields, predicatefns.Bind, Eval) and evaluates with time.Now(). A message type `when:` that uses today() can pass on Monday and refuse the whole file (degrading realtime) on a later reload. Fix: share one helper for building the message record and evaluating it, and either refuse today() in message `when:` used by triggers or evaluate only the date-independent part at load and rely on the runtime check (already present in systemWriter.post).'
severity: minor
resolution: 'Both checks now call realtimecfg.MessageType.Holds. The load check refuses a message when: that calls today() and evaluates the rest at a fixed time, so the result does not depend on the load date.'
status: addressed
---
