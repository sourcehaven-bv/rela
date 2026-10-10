import { test } from './fixtures';
import { KanbanPage } from '../pages';

// Count card fields (TKT-WA25G2): a relation shown as a number, and the
// number of comments on the card's entity.
test.describe('Kanban card counts', () => {
  test.afterEach(async ({ api }) => {
    const res = await api.listComments('feature', 'FEAT-002').catch(() => null);
    for (const c of res?.comments ?? []) {
      await api.deleteComment('feature', 'FEAT-002', c.id).catch(() => {});
    }
  });

  test('a card counts the entities related to it', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('feature-counts');

    await kanban.expectCardMetaCount('User Authentication', 'tasks', 1);
    await kanban.expectNoCardMetaCount('Dashboard Analytics', 'tasks');
  });

  test('a card counts its comments', async ({ appPage, api }) => {
    await api.addComment('feature', 'FEAT-002', 'first');
    await api.addComment('feature', 'FEAT-002', 'second');
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('feature-counts');

    await kanban.expectCardMetaCount('Dashboard Analytics', 'comments', 2);
    await kanban.expectNoCardMetaCount('User Authentication', 'comments');
  });
});
