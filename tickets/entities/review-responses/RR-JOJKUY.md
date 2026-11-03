---
id: RR-JOJKUY
type: review-response
title: Typing touch scans every entry of the process under the store mutex
finding: 'internal/realtime/typingstore.go:198 (touch calls sweepLocked over all channels on every call) plus nextDeadline (:317) on every start and channelFreeLocked (:234) on a cap refusal are O(total entries), up to MaxTypingPerChannel*MaxTypingChannels = 131072, all while holding typingStore.mu, which live() (:267) also takes on every Head and Read. Typing is a 1/s-per-channel call by design, so with a busy tenant every keystroke refresh does a full scan and serialises readers behind it. Bounded (needs many identities to fill; IdentityRate caps each one at about 30 live entries), so defense in depth only. Fix: drop the sweep from touch and let the expiry timer (Local.fire -> expire) own sweeping; in touch, treat an entry past its deadline in the target channel as absent and prune only that channel. Keep a per-store count of held channels instead of recomputing.'
severity: minor
resolution: touch prunes only its own channel and caps count only that channel; the expiry timer and Take sweep globally through the heap.
status: addressed
---
