---
id: RR-DDL5N1
type: review-response
title: 'Tautological tests: keyboard suppression and ActivityBar aria-hidden'
finding: |-
    Two tests passed against a broken implementation.

    (1) `PendingButton.test.ts` 'suppresses keyboard activation' asserted `emitted('click')` was undefined after `trigger('keydown')`. But @vue/test-utils never synthesises a click from keydown in jsdom regardless of preventDefault, so the assertion held with the ENTIRE onKeydown handler deleted. It tested nothing.

    (2) `ActivityBar.test.ts` asserted `aria-hidden="true"` on a statically-authored attribute in the idle state only — testing the template literal, and passing against a version that exposed itself to assistive tech the moment it became visible.

    Also flagged: no test covered pending+disabled together, and none covered two overlapping operations at production timings.
severity: minor
resolution: |-
    (1) Now dispatches a real cancelable KeyboardEvent and asserts `defaultPrevented`, which is the actual thing being suppressed. Mutation-verified: deleting the preventDefault call fails the test. Added the complementary case that an IDLE button does NOT prevent the default, so native Enter-activates behaviour is pinned too.

    (2) Now asserts aria-hidden in BOTH states — idle and visible — and additionally that no `role="status"` live region has been smuggled in alongside, since the route change is already announced by the destination view.

    The two coverage gaps are closed by tests added under RR-P0U6DI (overlapping operations at 500/400) and RR-17HODW (pending + disabled).
status: addressed
---
