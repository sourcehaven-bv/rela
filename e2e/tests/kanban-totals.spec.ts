import { test } from './fixtures';
import { KanbanPage } from '../pages';

// TKT-SVLQQ6: an enum column takes its colour from styles:, column_total puts
// the sum of a property under each heading, and a currency card field shows
// the amount as money. The locale is pinned so the amounts read one way.
test.use({ locale: 'nl-NL' });

test.describe('Kanban column totals', () => {
  test('shows column colours, totals and card amounts', async ({ appPage, api }) => {
    await api.createEntity('bugs', { properties: { title: 'Pipeline small', status: 'draft', estimate: 3500 } });
    await api.createEntity('bugs', { properties: { title: 'Pipeline large', status: 'draft', estimate: 8000 } });
    await api.createEntity('bugs', { properties: { title: 'Pipeline busy', status: 'in_progress', estimate: 1200 } });

    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-pipeline');

    await kanban.expectColumnColor('New', 'blue');
    await kanban.expectColumnColor('Fixed', 'green');

    // The seed bug in New has no estimate and adds nothing.
    await kanban.expectColumnSummary('New', /^Totaal: €\s11\.500$/);
    await kanban.expectColumnSummary('Fixed', /^Totaal: €\s0$/);
    // In Progress starts collapsed; its rail still shows the total.
    await kanban.expectCollapsedColumnSummary('In Progress', /^Totaal: €\s1\.200$/);

    await kanban.expectCardFieldText('Pipeline small', /^€\s3\.500$/);
  });
});
