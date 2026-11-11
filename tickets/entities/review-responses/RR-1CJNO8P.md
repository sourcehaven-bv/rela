---
id: RR-1CJNO8P
type: review-response
title: Stale read can notify after lock hand-over
finding: stop() did not cancel an in-flight read or the again path, so a tab that released the lock could still notify and write the seen set beside the new holder.
severity: significant
resolution: 'A generation counter bumped by start() and stop(); a read, its again path and every timer end without effect when the generation changed; in-flight tracking is per generation so a new holder''s baseline read is not held back. Test: drops a read that was on its way when the notifier restarted (mutation-checked).'
status: addressed
---
