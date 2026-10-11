import { test, expect, type ApiHelpers } from './fixtures';
import { EntityPage, ListPage, PilesPage } from '../pages';

// Piles (TKT-K3RJLH): a user's private, named stacks of entity references.
// The fixture server runs as e2e@example.com, so piles are available.

const TITLES = ['Pile walk alpha', 'Pile walk bravo', 'Pile walk charlie'];
const PILE = 'Friday review';

async function seedTasks(api: ApiHelpers): Promise<string[]> {
  const ids: string[] = [];
  for (const title of TITLES) {
    const created = await api.createEntity('tasks', { properties: { title } });
    ids.push(created.id);
  }
  return ids;
}

test.describe('Piles', () => {
  test('rows selected in a list become a pile that can be stepped through and edited', async ({
    appPage,
    api,
  }) => {
    const ids = await seedTasks(api);
    const list = new ListPage(appPage);
    const piles = new PilesPage(appPage);
    const entity = new EntityPage(appPage);

    // Select the three rows and make a new pile from them.
    await list.navigateToList('tasks');
    for (const id of ids) await list.selectRowById(id);
    await piles.chooseNewPile();
    await piles.expectNewPileItemCount(3);
    await piles.createPile(PILE);
    await piles.waitForToast(`Pile ${PILE} created with 3 items`);

    // The sidebar lists the pile with its count, and the panel its items.
    await piles.expectSidebarCount(PILE, 3);
    await piles.openPile(PILE);
    await piles.expectItems(PILE, TITLES);

    // Step through: the first item opens in pile scope, Next moves on.
    await piles.stepThrough(PILE);
    await entity.expectScopePosition(1, 3);
    await entity.scopeNext();
    await entity.expectScopePosition(2, 3);
    expect(new URL(appPage.url()).searchParams.get('from')).toBe('pile');

    // Remove one item from the panel, then undo the removal.
    await piles.openPile(PILE);
    await piles.tickItem(PILE, TITLES[1]);
    await piles.removeTicked(PILE);
    await piles.waitForToast(`1 item removed from ${PILE}`);
    await piles.expectItemAbsent(PILE, TITLES[1]);
    await piles.expectSidebarCount(PILE, 2);

    await piles.undo(`1 item removed from ${PILE}`);
    await piles.waitForToast(`1 item back on ${PILE}`);
    await piles.expectItems(PILE, TITLES);
    await piles.expectItemCount(PILE, 3);
    await piles.expectSidebarCount(PILE, 3);
  });
});
