---
id: RR-0QKRG
type: review-response
title: SupportsStreaming brittle against future decorators; silent buffered-fallback
finding: Type-switch in SupportsStreaming only recognizes *OsFS and *SafeFS. A future decorator (e.g. MeteredFS, TracingFS) causes silent buffered fallback — 500MB attachments would go from constant memory to 500MB resident. Debugging is miserable because there's no signal.
severity: significant
reason: 'Not actionable until a second decorator exists. Fix is ready (capability-interface-based SupportsStreaming), but applying it without a concrete second decorator is speculative. Follow-up ticket can lift the pattern when needed. Added SupportsStreaming caching (review finding #10) meanwhile.'
status: deferred
---
