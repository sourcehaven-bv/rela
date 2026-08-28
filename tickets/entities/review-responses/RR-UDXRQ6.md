---
id: RR-UDXRQ6
type: review-response
title: Flush after Close loses committed work with no record of what was lost
finding: 'Collector.Flush nils pending before attempting the enqueues, so if the queue is closed mid-shutdown the jobs are unrecoverable and the caller receives a joined error naming nothing. During shutdown this is a guaranteed path: a transaction commits, Close races in, the follow-up work evaporates and the operator cannot tell what vanished.'
severity: significant
resolution: 'Each failed enqueue is now wrapped with its job Kind (''jobs: flush "send-mail": ...'') before joining, so the log names the work that was lost. Not fully resolved: the jobs are still unrecoverable once flushed. Recovering them would require the collector to hold pending until each enqueue succeeds, which changes Flush''s idempotency contract — out of scope here and worth a follow-up if shutdown-time loss proves real in practice.'
status: addressed
---
