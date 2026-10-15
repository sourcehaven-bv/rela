import { test } from './fixtures';
import { EntityPage, KanbanPage } from '../pages';

// TKT-SVLQQ6: an enum column takes its colour from styles:, column_summary
// puts aggregate lines under each heading, and a property with
// `display: {currency: EUR}` shows as money on cards, in summaries and on the
// detail page. The locale is pinned so the amounts read one way.
test.use({ locale: 'nl-NL' });

test.describe('Kanban column summaries', () => {
  test('shows column colours, summaries and amounts', async ({ appPage, api }) => {
    const small = await api.createEntity('bugs', {
      properties: { title: 'Pipeline small', status: 'draft', estimate: 3500 },
    });
    await api.createEntity('bugs', { properties: { title: 'Pipeline large', status: 'draft', estimate: 8000 } });
    await api.createEntity('bugs', { properties: { title: 'Pipeline busy', status: 'in_progress', estimate: 1200 } });

    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-pipeline');

    await kanban.expectColumnColor('New', 'blue');
    await kanban.expectColumnColor('Fixed', 'green');

    // The seed bug in New has no estimate: it counts, but adds nothing.
    await kanban.expectColumnSummary('New', [/^Bugs: 3$/, /^Totaal: €\s11\.500$/, /^Grootste: €\s8\.000$/]);
    // An empty column: count and sum are 0, max has no value.
    await kanban.expectColumnSummary('Fixed', [/^Bugs: 0$/, /^Totaal: €\s0$/, /^Grootste: –$/]);
    // In Progress starts collapsed (one seed bug, no estimate); its rail shows
    // the summaries on one line.
    await kanban.expectCollapsedColumnSummary('In Progress', /^Bugs: 2 · Totaal: €\s1\.200 · Grootste: €\s1\.200$/);

    await kanban.expectCardFieldText('Pipeline small', /^€\s3\.500$/);

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', small.id);
    await entity.expectPropertyValue(/^€\s3\.500$/);
  });
});
