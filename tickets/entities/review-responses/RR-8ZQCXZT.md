---
id: RR-8ZQCXZT
type: review-response
title: Other cancel checks also silence deadlines
finding: localroleprime.go and three affordances warnings tested ctx.Err() == nil, which also hides DeadlineExceeded, contrary to the documented rule that a deadline is still logged.
severity: significant
resolution: 'Fixed: these checks now use errors.Is(ctx.Err(), context.Canceled).'
status: addressed
---
