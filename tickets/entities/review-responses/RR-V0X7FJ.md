---
id: RR-V0X7FJ
type: review-response
title: Hex encoding doubles URL size; large diagrams 414 silently
finding: ~h hex encoding doubles byte count; PlantUML servers/proxies cap URLs ~4-8KB, so a modest diagram silently 414s (broken img). Partially mitigated by the onerror fallback (RR-CIAI73) which restores the source on any failure including 414. A proactive size guard that degrades before emitting a doomed request is the cleaner fix; deferred together with the proxy approach (RR-proxy).
severity: minor
resolution: 'Mitigated by the onerror fallback (RR-CIAI73): an oversized hex URL that 414s now restores the source code block rather than showing a broken image. A proactive pre-request size guard is folded into the deferred proxy follow-up (RR-21O6D4), where a real error body can be returned.'
status: addressed
---
