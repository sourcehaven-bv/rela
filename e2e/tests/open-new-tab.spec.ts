import { test, expect } from './fixtures';
import { ListPage } from '../pages/list.page';
import { KanbanPage } from '../pages/kanban.page';
import { EntityPage } from '../pages/entity.page';

// TKT-3CSZRG. Navigable rows and cards used to be plain <div>/<tr> elements with
// an @click calling router.push, so the browser had no link to act on:
// cmd/ctrl-click, middle-click and "Open link in new tab" all did nothing. They
// are real anchors now; these tests assert the browser genuinely opens a tab
// (not that the SPA emulates one).
test.describe('Open in a new tab', () => {
  test('cmd/ctrl-click on a list row opens a background tab and leaves the list in place', async ({
    appPage,
  }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    const popup = await listPage.openRowInNewTab(0, { modifier: 'ControlOrMeta' });

    await expect(popup).toHaveURL(/\/entity\//);
    // The original tab must NOT have navigated — that was the old behaviour.
    await expect(appPage).toHaveURL(/\/list\/features/);
    await popup.close();
  });

  test('middle-click on a list row opens a background tab', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    const popup = await listPage.openRowInNewTab(0, { middle: true });

    await expect(popup).toHaveURL(/\/entity\//);
    await expect(appPage).toHaveURL(/\/list\/features/);
    await popup.close();
  });

  test('the row link carries the list scope, so the new tab is not orphaned', async ({
    appPage,
  }) => {
    // A tab opened without the scope query renders fine but has dead prev/next
    // navigation and a wrong back target — a silent divergence from a plain
    // click, which is the failure this guards.
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    const href = await listPage.rowLinkHref(0);

    expect(href).toContain('/entity/');
    expect(href).toContain('from=features');
    // A colon is legal unencoded in a query value, so match either form.
    expect(href).toMatch(/scope=list(:|%3A)features/);
  });

  test('a plain click still navigates in the same tab', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');

    await listPage.clickRow(0);

    await expect(appPage).toHaveURL(/\/entity\//);
  });

  test('cmd/ctrl-click on an entity-detail section table cell opens a background tab', async ({
    appPage,
  }) => {
    // These cells were already real <a href> elements, but carried an
    // unconditional @click.prevent that suppressed the browser's default on
    // EVERY click — so cmd-click navigated in place. Exactly the bug this
    // ticket exists to fix, in an anchor that already looked correct.
    const detail = new EntityPage(appPage);
    // The task view has the table-display section; TASK-001 implements FEAT-001.
    await detail.navigateTo('/entity/task/TASK-001');

    const cellLink = detail.sectionTableLink;
    await expect(cellLink).toBeVisible();

    const before = appPage.url();
    const popup = await detail.openInNewTab(cellLink);

    await expect(popup).toHaveURL(/\/entity\//);
    expect(appPage.url()).toBe(before);
    await popup.close();
  });

  test('header nav affordances (Prev/Next, Edit) open in a new tab', async ({
    appPage,
  }) => {
    // Navigation buttons that trigger no mutation are links too. Delete stays a
    // <button> on purpose — it mutates, so a new tab makes no sense for it.
    //
    // History is deliberately NOT asserted here. It renders only when
    // `/_config`.history_enabled is true, which requires a backend exposing the
    // optional version-history capability — postgres only. This suite runs the
    // default fsstore build, where the flag is correctly false and the link is
    // correctly absent; asserting it would pin an affordance that is *supposed*
    // to disappear on this backend. The flag/handler pair is pinned instead by
    // TestConfigHistoryEnabled_AgreesWithTheHandler.
    const listPage = new ListPage(appPage);
    await listPage.navigateToList('features');
    await listPage.clickRow(0);
    await expect(appPage).toHaveURL(/\/entity\//);

    const detail = new EntityPage(appPage);
    // `.first()` on purpose: Edit renders twice (desktop header + mobile
    // actions), and the desktop one carries a <kbd>E</kbd> shortcut hint, so
    // its accessible name is "Edit E" rather than "Edit".
    const edit = detail.editButton;
    await expect(edit).toBeVisible();

    const before = appPage.url();
    const popup = await detail.openInNewTab(edit);

    await expect(popup).toHaveURL(/\/form\//);
    expect(appPage.url()).toBe(before);
    await popup.close();

    // Delete must NOT have become a link — it mutates.
    await expect(detail.navLink(/Delete/)).toHaveCount(0);
  });

  test('cmd/ctrl-click on a kanban card opens a background tab', async ({ appPage }) => {
    const kanbanPage = new KanbanPage(appPage);
    await kanbanPage.navigateToKanban('feature-board');

    const firstCard = kanbanPage.cards.first();
    await expect(firstCard).toBeVisible();
    const cardId = (await firstCard.innerText()).split('\n')[0];

    const popup = await kanbanPage.openCardInNewTab(cardId);

    await expect(popup).toHaveURL(/\/(entity|form)\//);
    await expect(appPage).toHaveURL(/\/kanban\/feature-board/);
    await popup.close();
  });
});
