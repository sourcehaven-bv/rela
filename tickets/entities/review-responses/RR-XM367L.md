---
id: RR-XM367L
type: review-response
title: Relation view's bare URL publishes two frozen ordinals while the entity view publishes a live-relative link
finding: 'RelationHistoryView.defaultSelection() returns {base: secondNewest, target: newest} — two concrete ordinals. HistoryView returns {base: newest, target: ''current''}. So opening a bare URL and copying the address bar yields a live-relative link for entities and a frozen one for relations. The relation default''s own stated intent ("what the most recent edit changed", RelationHistoryView.vue:46) is live-relative by nature, so it stops meaning that the moment a new version lands. sideState() already resolves ''current'' to the newest version (RelationHistoryView.vue:123), so the plumbing exists. Also causes a single-version relation history to publish ?base=1&target=1 (v1 diffed against itself).'
severity: significant
resolution: 'Fixed. RelationHistoryView.defaultSelection() now returns {base: secondNewest, target: ''current''} — the sentinel, not the newest ordinal — so a bare relation URL publishes ?base=N&target=current, matching the entity view''s live-relative shape. sideState() already resolved ''current'' to the newest version, so no other plumbing changed. The <2-versions case now returns {current, current} instead of publishing ?base=1&target=1 (v1 against itself). Comment added at the function explaining why the sentinel is load-bearing here. Covered by a new e2e assertion: after loading a bare relation-history URL, expectUrlSelection(1, ''current'').'
status: addressed
---

## Finding

The headline feature is "share this diff", and it silently means two different
things depending on which history view produced the link.

| View | Bare URL publishes |
|---|---|
| Entity | `?base=3&target=current` (live-relative) |
| Relation | `?base=2&target=3` (frozen) |

The user's decision was that `current` stays live-relative. The entity view
honours it; the relation view does not, because `defaultSelection()`
(`RelationHistoryView.vue:52-59`) reaches for an ordinal where the sentinel is
the honest answer.

This is NOT the `current`-vs-`latest` labelling difference, which is deliberate
and documented — it's the default pair itself diverging.

## Impact

A relation link shared as "the most recent edit" stops meaning that as soon as a
fourth version is captured, silently. A single-version relation history
publishes `?base=1&target=1` — v1 diffed against itself.
