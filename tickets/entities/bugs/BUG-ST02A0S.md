---
id: BUG-ST02A0S
type: bug
title: Link style, phone layout, sidebar title and empty list text
description: Links ignore the theme link style, the phone layout loses the list tools and wastes space, the sidebar cuts off long names, and empty lists build a plural from the type name.
priority: medium
effort: m
why1: Each symptom has a local cause. UrlWidget styled its link with no colour and set the URL as title. RlPageHeader hid its tools slot below 768px. The closed drawer kept its box-shadow. The dashboard table had no phone layout. The menu button was raised. The workspace name used an ellipsis. The empty text appended "s" to the type name.
why2: The list moved its search, sort and filter into the header tools slot (4b8a94c2f). The slot was hidden on phones when it held only secondary actions, and that rule was not revisited.
why3: The API sent no label_plural, so the SPA had no plural label and built one from the type name. Pages and widgets were built and reviewed at desktop width.
why4: The phone layout is checked by hand, and the e2e mobile spec pinned the empty band as headroom instead of flagging it.
why5: No rule says that UI text derived from a schema uses the schema labels, or that a layout change is checked at phone width.
prevention: E2e tests pin that a list keeps its search box on a phone, leaves no empty band and casts no drawer shadow. Unit tests pin the link style and name, the dashboard phone labels and the plural label in empty and search text.
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

Several data-entry screens look broken or read badly.

- A `display: link` URL property with a link text uses the browser's default
link colour, not the theme link style. Its tooltip is the raw URL, so screen
readers read the URL as its description.
- On a phone the list page loses its search box, sort and filter. The page
leaves an empty band under the header, and a grey shadow strip shows at the left
edge where the closed drawer sits.
- On a phone, dashboard table cards scroll sideways, and short id-like values
such as `BUG-21N3J4N` break in the middle.
- The mobile menu button draws a raised box around its icon.
- A long app or workspace name in the sidebar is cut off with an ellipsis.
- An empty list says "No persons found.", built from the type name plus "s", and a list cannot set its own text.

## Acceptance Criteria

- [x] Links use the theme accent colour, underline on hover, and their accessible name is the link text.
- [x] A list on a phone shows its search, sort and filter, with no empty band and no shadow strip.
- [x] Dashboard table cards stack each row as label and value pairs on a phone; short values do not wrap.
- [x] The mobile menu button has no box.
- [x] A long workspace name wraps in the sidebar.
- [x] The empty list text and the search placeholder use `label_plural`, with a neutral text when it is not set.
- [x] A list can set its own `empty_message`.
- [x] The behaviour is documented.
