---
id: RR-KT3MBR
type: review-response
title: Focus after a one-click success moves away from where the user acted
finding: AC 8 always moves focus to the session header (`<section tabindex=-1>`, labelled 'Session <label>'). After a click on the deciding message's button, this (a) produces a second announcement (the section name, plus the role=status waiting line when the new state waits) next to the log notice the plan calls 'the one announcement', and (b) on a narrow or mobile panel can scroll the page away from the plan the user was reading. Prefer focusing the row the click came from (rows already have tabindex=-1 and RlChannel.jumpTo focuses them with preventScroll), and the header only for header clicks. RlChannelSession also has no focus style for the section.
severity: minor
resolution: Focus now returns to the row the click came from with preventScroll, or to the header for a header click, and the header section gets a focus-visible style.
status: addressed
---
