---
id: RR-69EK5AD
type: review-response
title: Ending an edit of a button-type message drops the live decision
finding: 'RlChannelComposer.vue current/selection watch: with a decision open (dismiss) and an edit of a message of another button type (contained), the edit''s end left selected=contained, which is not selectable. current fell back to pickable[0] and the selection watch reset to the default type before the editing watch restored the decision, so droppedKey was set, the strip went away and select(''text'') cleared the app''s form decision and its generation.'
severity: significant
resolution: current falls back to the live compose decision before the picker, and the selection watch resets to liveCompose()?.type before defaultType. Story EditButtonTypeKeepsDecision fails without the fix and passes with it.
status: addressed
---
