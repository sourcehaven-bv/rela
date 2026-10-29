---
id: RR-U2S51R
type: review-response
title: Load older with only hidden results gives no feedback
finding: 'The edge case accepts that loadOlder returning only hidden messages shows nothing new. The reader presses Load older and sees no change, every time, until a visible page is reached. AC 9''s hint covers only an empty log, and with hasOlder set its text ''Only hidden messages here'' can be wrong about older pages. Change the plan: render a run of hidden messages as a non-interactive line (''4 hidden messages'') that points to the toggle, or at least announce the count through the polite region. Reword the AC 9 text for the hasOlder case.'
severity: minor
resolution: When Load older adds only messages that do not render, the polite region announces the count and points to the toggle, and AC 9 has a separate text for the hasOlder case (AC 9, step 6). A non-interactive hidden line was rejected because it makes hidden a second collapsed.
status: addressed
---
