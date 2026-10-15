import { test } from './fixtures';
import { KanbanPage } from '../pages';

// TKT-8Z2LT3: a card field `{relation, direction, property}` shows that
// property of the related entities, rendered as the property renders: enum
// values as badges styled by `styles:`, a currency as an amount. The locale is
// pinned so the amount reads one way.
test.use({ locale: 'nl-NL' });

test.describe('Kanban related-property card fields', () => {
  test('shows the status of each implementing task once, as a styled badge', async ({ appPage, api }) => {
    const feature = await api.createEntity('features', { properties: { title: 'Related Status Feature', status: 'draft' } });
    await api.createEntity('features', { properties: { title: 'Lonely Feature', status: 'draft' } });
    for (const [title, status] of [
      ['Related Task A', 'in_progress'],
      ['Related Task B', 'done'],
      ['Related Task C', 'in_progress'],
    ]) {
      const task = await api.createEntity('tasks', { properties: { title, status } });
      await api.createRelation('tasks', task.id, 'implements', feature.id);
    }

    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('feature-task-status');

    await kanban.expectCardBadges('Related Status Feature', [
      { text: 'in_progress', color: 'orange' },
      { text: 'done', color: 'green' },
    ]);
    await kanban.expectNoCardFields('Lonely Feature');
  });

  test('shows the estimate of the fixed bug as an amount', async ({ appPage, api }) => {
    const bug = await api.createEntity('bugs', { properties: { title: 'Costly bug', status: 'draft', estimate: 3500 } });
    const task = await api.createEntity('tasks', { properties: { title: 'Fix the costly bug', status: 'draft' } });
    await api.createRelation('tasks', task.id, 'fixes', bug.id);

    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('task-bug-estimate');

    await kanban.expectCardFieldText('Fix the costly bug', /^Fix value:\s*€\s3\.500$/);
  });
});
