---
id: RR-B5X4ER
type: review-response
title: World mode re-ranks faces that the store already ranks
finding: 'internal/visibility/holders.go:133-148 (and :178-195) switches InWorld to AllFaces, then calls ResolveIDsErr / ListEntities per page to keep only the served face. The comment says ranking after the property test would serve a lower face. That contradicts the store contract (internal/store/graphquery.go, "Props and Narrowing test that prime", BUG-2SKLD3), which listPushdown relies on when it passes q.Faces through. The detour costs one extra gated resolve per page. Every non-served face also fills page slots, so a faced type needs more pages for the same number of ids. Fix: pass q.Faces through as composeReadScope does and drop `served`. If a real counter-example exists, pin it in a test and correct the store doc instead.'
severity: minor
resolution: The second face ranking is gone; faces pass through as on a list read; the POL-2 case pins the behaviour on three backends
status: addressed
---
