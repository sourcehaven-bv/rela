---
id: RR-3QQMAF
type: review-response
title: 'Q3: show since as text and document a since: state timer as the stall pattern'
finding: 'A title attribute is unreachable by keyboard and touch and unreliable for screen readers. The plan should render since as text (relative, computed client side from the absolute _busy since, so the live hash stays stable) in the indicator''s accessible text, with an optional visible "3h". Session timers already exist (SessionTimer with since: state, internal/realtime/session.go:190), so the docs should show a working-state timer that moves a stuck run to blocked after N hours. That is stall detection with no new code. A lease-based check stays a follow-up.'
severity: minor
resolution: 'The card shows the time in the state as visible text ("3h") and in the accessible name, computed in the browser from the absolute since so the live hash stays stable. The docs add a working-state timer with since: state that moves a stalled run to a blocked state.'
status: addressed
---
