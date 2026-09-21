---
id: RR-NNFWGP
type: review-response
title: The terms quality floor rejects every prefix match, blanking the menu mid-typing
finding: 'The plan states the floor `info.terms[o] >= uf.split(query).length` as verified fact. It was validated only on complete-word queries. uFuzzy''s `terms` counts EXACTLY-matched terms — a term whose left AND right both land on a word boundary — so a term matching a PREFIX of a haystack word contributes 0. Measured against @leeoniya/ufuzzy 1.0.19: `fanc` -> terms=0 (rejected), `fancyrep` -> terms=0 (rejected), `fancy-som` -> terms=1 of 2 (rejected), and only the complete `fancy` / `fancy-some` pass. Two consequences. (a) The menu goes BLANK while typing: the query is re-sent each keystroke, so f/fa/fan/fanc all show ''No matches'' and the entity pops into existence only on the character completing a whole word — worse than today''s behaviour, and no test in the plan''s table would catch it because every fixture uses complete words. (b) It breaks the existing ID-prefix contract: measured, query `TKT-AB` gives TKT-ABCD terms=1 of 2, so TKT-ABCD is DROPPED from the results. That is the exact assertion in useMentionMenu.test.ts:24-29, and it makes the plan''s own AC 4 (''an ID prefix still beats a loose title match'') contradict AC 5 (''junk rejected'') through one threshold.'
severity: critical
resolution: The `terms >= need` floor is removed. With a picked type already bounding the candidate set, an aggressive client-side floor is no longer load-bearing, so the ranker RANKS rather than filters and relies only on uFuzzy's own filter() for exclusion. AC 7 adds a prefix matrix (every prefix of a target query must keep the target visible) as a standing guard, so any floor added later by feel cannot silently reintroduce this.
status: addressed
---

## Evidence

Measured against the installed `@leeoniya/ufuzzy` 1.0.19:

```
query        need  terms  verdict    haystack
fanc         1     0      REJECTED   FancyReport some word in title
fancy        1     1      KEEP       FancyReport some word in title
fancyrep     1     0      REJECTED   FancyReport some word in title
fancy-som    2     1      REJECTED   FancyReport some word in title
fancy-some   2     2      KEEP       FancyReport some word in title
TKT-AB       2     1      REJECTED   TKT-ABCD ticket about things
TKT-AB       2     2      KEEP       TKT-AB short one
TKT-ABC      2     1      REJECTED   TKT-ABCD ticket about things
```

uFuzzy's own type definitions describe `terms` as the number of
**exactly-matched** terms (intra = 0) where **both** left and right landed on a
`BoundMode.Loose` or `Strict` boundary. A prefix has no right boundary.

## Why this is the most expensive finding

An autocomplete menu is typed one character at a time. A floor that only admits
completed words means the menu is empty for most of the interaction and the
result appears to flicker into existence. This is a regression against the
current behaviour, where `rankByIdMatch` never removes anything
(`useMentionMenu.ts:63` returns early on an empty query and otherwise only
reorders).

It also silently breaks `useMentionMenu.test.ts:24-29`, which asserts `TKT-AB` →
`['TKT-AB','TKT-ABCD','XTKT-AB','OTHER']`. Under the floor, `TKT-ABCD` is not
reordered — it is gone.

## Required plan change

The floor must tolerate a prefix on the **final** needle term, since that is the
term the user is still typing. uFuzzy has no built-in for this. Options:

- `terms >= need - 1`, plus an explicit check that the final term is a genuine
prefix of a haystack word (available from `info.ranges`).
- Or drop `terms` and threshold on a different counter (`intraIns` / `chars`),
re-measured on prefix queries rather than complete words.

Whichever is chosen, the test matrix must cover **every prefix of a target
query** (`f`, `fa`, `fan`, `fanc`, `fancy`, `fancy-`, `fancy-s`, …), not just
the completed form, plus the `TKT-AB` → `TKT-ABCD` case explicitly. The plan's
current test table cannot detect this class of bug.
