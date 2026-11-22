---
id: BUG-JRJDSVJ
type: bug
title: Long property values on phones; HTML answers taken as API data
description: A long value without spaces in a properties section runs past a phone screen and scrolls the page sideways. The API client hands a 200 HTML page (a proxy sign-in page) to components as data, which then fail on it.
priority: medium
effort: s
why1: 'The properties value boxes may shrink (min-width: 0) but nothing lets the text inside break, so a word longer than the row overflows it. The API client returned whatever body a 2xx carried, and axios hands a non-JSON body on as a string.'
why2: Only long-form values (property-long) got overflow-wrap; short text fields were assumed to hold words. The response interceptor only checked for the provisioning answer, because rela itself always answers its API with JSON.
why3: Phone layouts were checked with seed data of short words, and a proxy that answers with its own page was not part of the client design.
why4: The e2e mobile spec had no case with a long unbroken value, and no unit test sent a non-JSON body through the client.
why5: Wrapping is decided per surface rather than once for every value, and the client trusted the transport to deliver JSON instead of checking what it got.
prevention: An e2e test on the phone viewport checks that a long unbroken value stays inside both kinds of properties section and that the pane does not scroll sideways. A unit test sends HTML, JSON, empty and blob answers through the real client.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found in saas-dev.

- In a `display: properties` section, a long value without spaces (an ID such as `org_01J…`) does not wrap. At 375 pixels its `span.display-value` runs past the screen and the main pane scrolls sideways.
- After a proxy redirect, an API call can be answered with a 200 HTML page, for example a sign-in page. The API client returns it as data, and components fail with errors such as `commands.value.map is not a function` in EntityDetail.vue.

## Acceptance Criteria

- [x] A long unbroken value in a properties section wraps within its row, in display and inline-edit sections.
- [x] The main pane does not scroll sideways because of such a value.
- [x] An HTML answer to a JSON API call rejects with an ApiError that tells the user to reload; components never receive the HTML as data. It also asks the session-ended probe (BUG-9DR9867) whether the session ended.
- [x] Empty bodies, JSON bodies and calls that ask for another response type (blob downloads) keep working.
