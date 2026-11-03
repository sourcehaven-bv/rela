---
id: RR-ODTNR5
type: review-response
title: Skipping the own guard loses the known-outcome rule, and a compose Send can land somewhere else
finding: 'Today a button is loaded only when its own guard holds for the preset, so a click''s outcome is known (internal/realtimecfg/buttons.go:24-28, 44-46, 264-281). The plan skips that check when the own guard reads a left-out property (Approach 4). The decider then tries later entries when the guard does not hold for what the person typed (internal/realtime/session.go:584-597). With `[{to: drafting, when: "len(message.feedback) >= 20", button: {compose: true}}, {to: approved, outputs: [...]}]`, a short note sent from "Request changes" approves the plan and runs its outputs. If no entry applies, the message is stored, the session stays, and `sendComposer` ends the form decision silently (frontend/src/components/entity/channels/useSessionDecisions.ts:232-249). Fix: (a) refuse a compose button when a later entry admits the same roles and could take the post, or warn when the own guard reads a left-out property and a later catch-all exists. (b) In the SPA, compare the session state in the post answer with the button''s `to`, and show a toast (new en/nl strings) when they differ. (c) Update the godoc of TransitionButton and SessionButtonView. Add an AC and a test for both outcomes.'
severity: significant
resolution: ADDRESSED The own guard may no longer read a left-out property, and an earlier guard that admits a clicker's role may not either; both are refused at load. Given props are locked, so every accepted Send takes the button's entry and no SPA toast is needed (AC 4, AC 7, Approach 4; godoc of TransitionButton, SessionButtonView and ChannelSessionButton updated).
status: addressed
---
