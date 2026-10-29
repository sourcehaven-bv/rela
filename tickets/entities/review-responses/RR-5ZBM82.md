---
id: RR-5ZBM82
type: review-response
title: 'Stale _actions window: decisions can briefly offer a type the caller cannot move'
finding: 'In useChannel.follow, setChannel(page.channel) puts the new session (state, generation, transitions) on the head before refetchHead brings the new _actions. composerTransitions intersects stale _actions.transitions with the new state''s on:. The generation lock is already released at that point (generation != decidedGen), so a type that appears in both states'' on: shows briefly as a decision, possibly for a role the caller lacks in the new state. The composer marks have the same gap today, but a one-click button turns it into a real post. Also, decisionViews should check head.channel.state === ''open'' and !session.final itself, instead of trusting _actions.transitions, which lags a live close by one head fetch. Not an issue for the PoC''s states; worth one unit-table row.'
severity: minor
resolution: decisionViews now checks channel.state open and not final itself and requires actionsGeneration equal to the session generation, which closes the stale _actions window, with unit-table rows for a closed head with stale actions.
status: addressed
---
