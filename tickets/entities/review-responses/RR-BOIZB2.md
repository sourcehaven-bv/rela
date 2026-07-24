---
id: RR-BOIZB2
type: review-response
title: HelpModal loadHelp concurrency race (stale mermaid render)
finding: loadHelp has no request cancellation; the watch fires on both props.open and props.entityType. Switching help target (or fast open/close/open) starts a second loadHelp before the first axios.get resolves — two flows race to set helpContent and both call renderMermaidDiagrams against whatever DOM is current, risking a stale/mismatched diagram.
severity: significant
resolution: 'Added a monotonic loadToken: each run captures its token and bails (before writing content, on error, and after nextTick before rendering) the moment a newer run starts. A superseded load never overwrites content or renders mermaid against the newer entity''s DOM.'
status: addressed
---
