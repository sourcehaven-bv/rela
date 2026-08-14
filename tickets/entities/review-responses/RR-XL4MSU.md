---
id: RR-XL4MSU
type: review-response
title: Card data keyed by array index binds one card's count to another card's tile
finding: |-
    cardData in DashboardView.vue was a Map keyed by the v-for array index, and :key was the index too. If the card list changes shape while the view is mounted, a survivor renders a dropped card's data. Reviewer proved it: cards A(count 100) and B(count 200), swap the list to just [B], and B renders 100 — A's number, presented as fact.

    Pre-existing, and the window is narrow (no <KeepAlive>, so a route remount re-runs loadData; SSE refresh does not reload the schema store). But this ticket makes the card list PER-PRINCIPAL, which is a new and legitimate reason for it to change shape between loads — so the latent bug got a live trigger.
severity: minor
resolution: |-
    Keyed cardData and :key on a content-derived identity (cardKey) instead of position. Cards carry no configured id, so the key is built from title+query+display: title alone is not guaranteed unique and query alone is genuinely shared by cards displaying the same data differently.

    Built with JSON.stringify([...]) rather than string concatenation, so the parts cannot run together — {title:'a b', query:'c'} and {title:'a', query:'b c'} must not collide into one key.

    Pinned by a new DashboardView test, 'never binds one card's data to another card's tile', which reproduces the reviewer's exact A/B scenario. Mutation-verified: restoring index keying fails it.

    Found while fixing this: my first cardKey used a template literal whose separators were literal NULL bytes, not spaces — invisible in the editor and in Read output (both render them as spaces), and it silently defeated my own mutation testing (the perl/grep anchors never matched, so a 'passing' mutation run was actually a no-op). The JSON.stringify form removes the whole class of problem.
status: addressed
---
