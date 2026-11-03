---
id: RR-GTUMFMQ
type: review-response
title: Network sends run synchronously at mark sites
finding: Mark functions take no context; some run under the per-channel outputs lock; notePosted sends two hints; each write takes an extra pool connection on postgres.
severity: significant
resolution: Marks only enqueue. A sender goroutine with a bounded queue merges over about 20 ms and sends; overflow becomes a resync hint. Flush and Close added; channel write CLI commands call Flush before exit.
status: addressed
---
