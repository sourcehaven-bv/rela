---
id: RR-FB1O
type: review-response
title: 'Minor nits N1-N6'
finding: |
  - N1: Approach #2 "extract sub-render OR use v-if mode!=='display'" is overthinking; existing `v-else` already catches inline-edit (would be moot with RR-FB1F dropping the widening anyway).
  - N2: Risk section depends on lastSeenServer access (resolved via RR-FB1A) and AC 5 surface (resolved via RR-FB1C).
  - N3: AC 7 mentions togglingIndices — symbol no longer exists post-IHC7A. Replace with reference to `contentAutoSave` in EntityDetail.
  - N4: Verify IHC7A's PR #912 has merged before starting IHC7B implementation.
  - N5: Bang-cast on routingHint — vanishes naturally after RR-FB1H (discriminated union).
  - N6: Per-widget round-trip is no-op for 7 of 8 widgets — moot after RR-FB1F (no widget changes at all).
severity: nit
status: addressed
resolution: |
  N1, N5, N6 dissolve with RR-FB1F (no widget changes). N3 updated in PLAN. N2 dissolves with C1/C3 resolutions. N4 captured as an explicit gate at the top of PLAN's implementation phase: "Confirm PR #912 has merged into develop and the IHC7B branch is rebased on the merged tip before starting."
---
