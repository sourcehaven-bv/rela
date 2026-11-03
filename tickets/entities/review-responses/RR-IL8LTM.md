---
id: RR-IL8LTM
type: review-response
title: Mobile, screen reader context and iOS keyboard are not covered
finding: 'Accessibility: focus jumps to Feedback. The composer shows the type label (Revise), not the button label (Request changes), and nothing says that Topic was filled in for the person. Add a polite live status or an aria-describedby on the form that names the decision, through rela-components useMessages with en/nl text. Mobile: on iOS Safari, focus() after awaits outside the tap handler does not open the keyboard (openForm awaits a confirm and nextTick, useSessionDecisions.ts:399-409). The composer may also be off screen. Add an AC or a mobile-layout E2E check that the composer scrolls into view, and note the keyboard limit.'
severity: minor
resolution: ADDRESSED The strip names the button label and given values, is a polite status region, and the send button's aria-describedby names it. The focused field scrolls into view, the E2E asserts it is in the viewport, and the iOS keyboard limit is noted in the header comment (AC 8, AC 12, Approach 8).
status: addressed
---
