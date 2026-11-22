---
id: BUG-1MEXGMG
type: bug
title: Flow run page repeats the flow and shows person ids and wrong wording
description: The run page names the flow twice and lights Overview; the timeline shows person ids and says Waits for no one and 1 failed attempts.
priority: medium
effort: s
why1: The run page put the flow label in the page title and the run card's heading; the timeline showed the raw actor id; the waiting text showed the no-roles line beside a failed job; the failed-attempts text had one form for every count; the sidebar lit a space home row on every page under it by prefix match.
why2: The run card was built for the run list and the subject's page and was reused on the run page as it was; flow events carry only the actor id; the plural forms of vue-i18n were not used for this string; the prefix match treats a space home path as the parent of every page.
why3: The tests checked what the card shows in a list and that the server names subjects but not actors; no test covered a failed job's waiting text with a count of one or the sidebar on a page that only shares the space prefix.
why4: The flow pages were built screen by screen and checked by hand with sample data that used logins as actors and a count above one.
why5: No test looks at the run page as a whole with realistic data (person ids as actors; one failed attempt).
prevention: TestFlowControl_TimelineNamesActor pins the actor title; flowView and FlowRunCard tests pin the actor name and the singular and plural wording and the hidden title; the sidebarNav test pins a space home row.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

In the harbourmaster portal, the flow run page and its timeline:

- named the flow twice: as the page title and as the card heading;
- lit "Overview" in the sidebar on every page of the space;
- showed timeline actors as person ids (PSN-...);
- said "Waits for no one" on a run whose job had failed;
- said "after 1 failed attempts".

## Expected

The flow is named once, the sidebar lights no row it does not lead to, actors
show by name when the reader may see them, a failed job does not claim the step
waits for no one, and the count reads in the singular for one.
