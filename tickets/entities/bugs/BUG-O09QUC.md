---
id: BUG-O09QUC
type: bug
title: 'Inline entity-reference picker: backspace kills completion + ID-prefix search ranking is poor'
description: 'The markdown editor''s backtick-triggered entity-reference autocomplete (BacktickAutocompletePopup / useBacktickAutocomplete) had two issues. (1) Backspace killed completion: once the picker resolved an ID prefix and entered phase ''id'', backspacing back across the prefix boundary left the session stuck in phase ''id'', feeding the now-partial prefix into the entity search as free text instead of handing control back to the prefix machine. (2) Poor relevancy: typing an ID prefix like ''VAD-ACT-'' surfaced loosely-matching titles above the entities whose IDs actually start with that prefix. Verified against the live tickets/ corpus — before the fix a ''BUG-'' search returned zero BUG-* entities in the top 8 (all RR-* whose titles contain the word ''bug'').'
priority: medium
effort: s
why1: Once in phase 'id', useBacktickAutocomplete only advanced the phase machine forward; backspacing across the resolved-prefix boundary never returned control to the prefix machine, so the partial prefix was searched as free text.
why2: applyTypedToPhase only handled the prefix→id transition, not the symmetric id→prefix retreat, so the backspace direction was unhandled. Separately, the bleve 'id' field is keyword-analyzed (whole ID = one token) but the only ID query was an exact term query, so a partial prefix matched nothing and fuzzy title tokens dominated ranking.
why3: The phase machine was written for the forward typing path only, and the search backend had no prefix query on the id field — partial-ID matching was never a modeled case for either layer.
why4: Tests exercised forward typing and exact full-ID search (TestIndex_SearchByID), but neither a backspace-across-prefix sequence nor a partial-ID-prefix ranking case, so the gap was invisible in CI.
why5: 'Systemic: incremental-search UIs need symmetric edit handling (every forward transition needs its reverse) and ID-prefix relevance is a first-class query shape, not an afterthought — both layers modeled only the happy path and lacked tests for the inverse/partial cases.'
prevention: 'Added regression tests on both layers: a backspace-across-prefix recovery test in useBacktickAutocomplete.test.ts and TestIndex_SearchByIDPrefix in bleveindex_test.go (reproducing the exact VAD-ACT- scenario, asserting id-prefix matches occupy the top ranks). The durable guard is the bleveindex id-prefix test, which fails CI if id-prefix ranking regresses.'
status: ready
---

## Symptom

Two complaints about the markdown editor's  ` -triggered entity-reference picker
(the "Entities matching VAD-ACT-…" popup):

1. **Backspace kills search completion** — after the picker resolves a prefix,
backspacing a character makes the dropdown go empty / stop completing.
2. **Poor search relevancy** — typing an ID prefix like `VAD-ACT- ` ranks
loosely-matching titles above the entities whose IDs start with the prefix.

## Root cause

- **Frontend** (`useBacktickAutocomplete.ts `): the phase machine
(`prefix → id `) only moved forward. Once in phase `id `, backspacing back
across the prefix boundary never handed control back to the prefix machine, so
the partial prefix was fed into entity search as free text.
- **Backend** (`bleveindex.go `): the `id ` field is keyword-analyzed (whole ID
is one token), but the only ID query was an **exact term** query. A partial
prefix matched nothing, so fuzzy title tokens dominated. Probed against the real
`tickets/ ` repo, a `BUG- ` search returned **zero `BUG-* ` entities in the top
8** — all `RR-* ` whose titles contain the word "bug".

## Fix

- Symmetric backspace: phase `id ` hands control back to the prefix machine when
typed text retreats across the prefix boundary.
- `rankByIdMatch `: client-side tier ranking (exact ID → ID-prefix → substring →
title-only), backend-agnostic — the autocomplete sends a prefix-stripped body
query and the three backends each rank differently.
- Bleve: added a **prefix query** on the `id ` field (boost 6.0) + split the
exact-ID boost to 8.0; helps full-prefix callers (card picker, command palette,
raw `/_search `).

## Fixed by

PR sourcehaven-bv/rela#1031.
