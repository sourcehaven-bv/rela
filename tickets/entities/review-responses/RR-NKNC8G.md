---
id: RR-NKNC8G
type: review-response
title: AC 5 cannot stay green without edits; new view fields break exact toEqual cases and leak app fields into the library type
finding: 'The decisionViews tests compare whole views with toEqual (frontend/src/components/entity/channels/channelView.test.ts:986, 1004-1005, 1115-1118, 1184, 1189, 1213 carry confirm: true / danger: true). If decisionTone sets endsSession: true on every final target (Approach 7, plan line 384), or primary: false anywhere, every one of those cases fails, so AC 5''s "stay green without edits" and the risk mitigation at plan line 575 are false as written. Separately, the plan adds confirmText and endsSession to rela-components ChannelDecisionView (frontend/packages/rela-components/src/components/channel/types.ts:336-354) although the library never reads them; the library already ignores confirm, and its doc ("Ask before posting: the move ends the session", types.ts:347-350) becomes wrong under this ticket. Change the plan: keep app-only dialog fields on an app-side type (export interface AppDecisionView extends ChannelDecisionView in presetViews.ts), set them only when an option is authored (for example set movesOnly only for an authored confirm into a non-final target), never emit false/undefined-valued keys, update the confirm doc in types.ts, and add only primary to the library.'
severity: significant
resolution: App-only dialog fields (confirmText, movesOnly, outcomeDanger) move to AppDecisionView in presetViews.ts and are set only when an authored option causes them, never as false or undefined. AC 6 now explains why the existing toEqual cases stay green, the library gains only primary, and its confirm and danger docs are reworded.
status: addressed
---
