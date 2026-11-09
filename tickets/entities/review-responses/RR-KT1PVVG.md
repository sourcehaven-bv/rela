---
id: RR-KT1PVVG
type: review-response
title: 'Security: header-mode callers without identity share one gate key'
finding: In header mode callers without the identity header are all user unknown and share one bucket; a proxy that passes a client-chosen header lets each name get a fresh bucket.
severity: minor
resolution: 'Superseded: the peer-key fix from TKT-H91N91L was removed by TKT-TB2B8N5 (DEC-QKLMMD4). See reason.'
reason: 'DEC-QKLMMD4: the gate is keyed on the resolved principal, as for any caller. Header-mode callers without identity share one key by design; a sync client gets its own identity at the edge. The peer-key resolution of TKT-H91N91L was removed by TKT-TB2B8N5.'
status: wont-fix
---
