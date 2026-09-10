---
id: RR-ZF0FNT
type: review-response
title: Two-note banner layout is visually unverified; .banner-note CSS is deletable with no test failure
finding: 'WorldBanner.vue:38 wraps the slot in an inline <span class="world-banner__note"> carrying `flex: 1 1 24rem`, a basis tuned (per its comment at :69) for one wrapping text run. The new .banner-note { display: block } children rely on flex-item blockification and change the intrinsic height to two stacked blocks. jsdom does no layout, so none of the 2452 frontend tests can see this: deleting the entire .banner-note CSS block passes 67/67. The one genuinely new UI arrangement in this ticket is unverified by automation and needs a real browser at narrow width with a world banner label plus both notes.'
severity: significant
resolution: 'Verified by hand in a real browser, measuring rendered geometry rather than eyeballing. Fixture: both keys declared long enough to wrap, beside a world declaring a `banner:` label. Full width — wrapper computes to display:block (flex-item blockification confirmed), both notes full-width, 18px each, 4px gap, no overlap. Below the 24rem flex basis (forced to 420px) — wrapper wraps under the label exactly as that basis intends (wrappedUnderLabel: true), each note takes two lines (h:36), stacked without overlap, banner grows to 121px containing them with nothing clipped. Recorded the result in the CSS comment, including that jsdom sees none of this and that the block display relies on flex-item blockification, so the next person does not re-derive it.'
status: addressed
---
