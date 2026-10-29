---
id: RR-D58HY2
type: review-response
title: Decision-button docs still describe the old per-type rule
finding: 'docs-project/entities/guides/GUIDE-realtime.md:1856-1866. The type button is now computed from the caller''s listed entries without a button (channelView.ts decisionViews, presetViews.ts:77). It opens the form when one of those entries is guarded or an earlier person-fired entry is guarded. Its confirm and danger come only from those entries. This changes existing branching configs that have no button: a lead whose only listed entry is an unguarded lead-only edge now gets a one-click decision where it used to get the composer. The section still says ''a condition, or moves to several states'' about the type as a whole. Fix: reword the bullets in terms of ''the entries your post could take (_actions.entries) that have no button'', and add the earlier-guarded-entry rule.'
severity: minor
resolution: Reworded the Decision buttons bullets around the entries in _actions.entries that have no button, added the rule about a guarded earlier person-fired entry, scoped the confirm rule to the entries the button stands for, and added a bullet documenting that a branching type without buttons can now be one-click for a caller whose only listed entry is an unguarded edge to one state.
status: addressed
---
