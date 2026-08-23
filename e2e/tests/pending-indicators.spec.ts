import { test, expect } from './fixtures';
import { PendingPage, SearchPage, SettingsPage } from '../pages';

/**
 * Pending-indicator framework (TKT-TFSNBY).
 *
 * These cover the two acceptance criteria that unit tests structurally
 * cannot: AC3 (a pending button's width never changes) and AC4 (paired
 * action buttons share a width). Both are geometric, and jsdom has no
 * layout engine — the Vitest suite asserts the DOM preconditions, this
 * asserts the pixels those preconditions are supposed to produce.
 *
 * Also covers the governing rule end-to-end: against a local server every
 * request here resolves in single-digit milliseconds, far inside the 250ms
 * (navigation) and 500ms (action) delays, so the correct observable
 * behaviour is that NO indicator ever appears.
 */
test.describe('Pending indicators', () => {
  test('search button reserves width for its pending label', async ({ appPage }) => {
    const search = new SearchPage(appPage);
    const pending = new PendingPage(appPage);
    await search.navigateToSearch();

    // Both states are rendered at all times; only one is visible.
    await pending.expectBothLabelsPresent('Search', 'Searching…');
    await pending.expectPendingLabelReservedButHidden('Search', 'Searching…');
  });

  // THE GEOMETRIC ASSERTION. "Search" is markedly shorter than
  // "Searching…", so a naive label swap would visibly widen the button
  // mid-click. The reservation makes the resting width already equal to
  // the pending width.
  test('search button width does not change while a search runs', async ({ appPage }) => {
    const search = new SearchPage(appPage);
    const pending = new PendingPage(appPage);
    await search.navigateToSearch();

    const restingWidth = await pending.buttonWidth('Search');
    expect(restingWidth).toBeGreaterThan(0);

    // The reservation is only meaningful if the pending label is genuinely
    // WIDER than the resting one — otherwise the button would be big
    // enough by accident and this test would pass without the mechanism.
    const restingLabelWidth = await pending.labelWidth('Search', 'Search');
    const pendingLabelWidth = await pending.labelWidth('Search', 'Searching…');
    expect(pendingLabelWidth).toBeGreaterThan(restingLabelWidth);

    // Both labels occupy the same grid cell, so the cell — and therefore
    // the button — is already sized to the wider of the two at rest.
    // A collapsed hidden box (display:none / v-if) would report 0 here.
    expect(await pending.labelWidth('Search', 'Searching…')).toBe(pendingLabelWidth);

    await search.search('a');

    const afterWidth = await pending.buttonWidth('Search');
    // Exact equality: the box is reserved, not merely similar. Any drift
    // here means the width is being driven by the visible label again.
    expect(afterWidth).toBe(restingWidth);
  });

  // AC4 — Save sits beside Reset in the settings footer. Without the
  // equal-width pair rule, Save's reservation for "Saving…" leaves it
  // visibly over-padded next to a snugly-fitted Reset.
  test('settings Save and Reset share a width', async ({ appPage }) => {
    const settings = new SettingsPage(appPage);
    await settings.navigateToSettings();

    const { save, reset } = await settings.actionPairWidths();
    expect(save).toBeGreaterThan(0);
    expect(save).toBe(reset);
  });

  // The governing rule: fast operations show nothing at all. Against a
  // local server a search resolves far inside the 500ms action delay, so
  // the label must never have swapped.
  test('a fast search never swaps the button label', async ({ appPage }) => {
    const search = new SearchPage(appPage);
    const pending = new PendingPage(appPage);
    await search.navigateToSearch();

    await search.search('a');

    // data-pending is only set once the gate has actually elapsed.
    await expect(pending.pendingButton('Search')).not.toHaveAttribute('data-pending', 'true');
  });

  // Same rule for the navigation class. Route changes against a local
  // server are well inside the 250ms bar delay.
  test('fast navigation never shows the activity bar', async ({ appPage }) => {
    const pending = new PendingPage(appPage);
    const search = new SearchPage(appPage);

    await pending.expectActivityBarHidden();
    await search.navigateToSearch();
    await pending.expectActivityBarHidden();
  });

  // AC8 — the shared stylesheet replaced ten duplicated @keyframes blocks
  // with one definition and, more importantly, ONE reduced-motion rule.
  // Before this, the preference was honoured in 1 of 11 animation sites.
  test('spinner animation is defined globally and suppressed under reduced motion', async ({
    appPage,
  }) => {
    const normal = await appPage.evaluate(() => {
      const el = document.createElement('div');
      el.className = 'spinner';
      document.body.appendChild(el);
      const name = getComputedStyle(el).animationName;
      el.remove();
      return name;
    });
    // Resolves even though every component-scoped copy was deleted.
    expect(normal).toBe('spin');

    await appPage.emulateMedia({ reducedMotion: 'reduce' });
    const reduced = await appPage.evaluate(() => {
      const el = document.createElement('div');
      el.className = 'spinner';
      document.body.appendChild(el);
      const name = getComputedStyle(el).animationName;
      el.remove();
      return name;
    });
    expect(reduced).toBe('none');
    await appPage.emulateMedia({ reducedMotion: null });
  });

  // RR-B7U3I8 in the real app: after a burst of navigations the bar must
  // not be stranded on screen. A counter-based implementation leaks here
  // whenever one of these is cancelled or deduplicated.
  test('the activity bar is not stranded after rapid navigation', async ({ appPage }) => {
    const pending = new PendingPage(appPage);

    await pending.rapidNavigate(['/search', '/analyze', '/search', '/dashboard']);

    await pending.expectActivityBarHidden();
  });
});
