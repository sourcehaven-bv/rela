---
id: RR-EI4YU6
type: review-response
title: The SPA cannot tell role-split edges apart, so the hint and buttons are misleading for AC 3
finding: 'session.transitions carries no roles (SessionTransitionView, session.go:102-113), by design. For [{to: escalated, roles: [lead]}, {to: noted}], a member sees a form decision with no toLabel and the hint "Working or Planning, depending on the message", although for them it is a one-click move to `noted` that does not depend on the message. Either accept this and say so in the plan, or filter edges per caller using per-caller info already in _actions (for example add the reachable `to` per type to _actions.transitions as a later wire change). At minimum, the hint wording should not claim "depending on the message" when the edges differ by poster rule.'
severity: minor
resolution: Accepted that role-split edges show to every caller (recorded in scope out, Q5 and risks), and the hint wording became "may move" so it stays true.
status: addressed
---
