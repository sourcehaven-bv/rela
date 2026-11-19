---
id: BUG-GPWPW14
type: bug
title: An empty enum reads None where other empty fields read Empty
description: On an entity page an empty enum (Country) reads None while an empty string field reads Empty.
priority: low
effort: xs
why1: The enum pickers in InlinePropertyValue used the app's common.none text while the swap read view used the library emptyValue text.
why2: The pickers were added later and took a placeholder from the app i18n file instead of the wording the library already showed.
why3: Two text sources exist for the same state (app i18n and library chrome text) and nothing ties them together.
why4: Tests checked each treatment on its own; none compared how treatments word the same empty state.
why5: Shared UI wording has no single owner per state.
prevention: InlinePropertyValue.test.ts asserts that an empty enum, enum list and string read the same and that the empty option uses that wording.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

On an entity page the user may edit, an empty string field reads "Empty" but an
empty enum or enum list (Country) reads "None". The same state has two names on
one page.

## Expected

Every empty value reads the same, and the option that clears an enum uses that
wording too.
