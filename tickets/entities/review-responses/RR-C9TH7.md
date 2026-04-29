---
id: RR-C9TH7
type: review-response
title: + Filter button shows misleading F hotkey hint in EntityList
finding: AdHocFilterMenu defaults buttonHotkey to F. SearchView wires f to filterMenuRef.value?.open(). EntityList does NOT bind f. The list view shows a kbd F that does nothing.
severity: significant
resolution: Added onOpenFilter callback to useListKeyboard bound to 'f', and EntityList wires it to filterMenuRef.value?.open(). The kbd F hint on the + Filter button now does what it advertises in both list and search views.
status: addressed
---
