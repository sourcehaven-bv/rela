import { test } from './fixtures';
import { KanbanPage } from '../pages';

// TKT-83S9G0Z: `cell_colors:` paints the cells of a swimlane board as a heat
// map. Each toned cell shows its label as text, so colour is not the only cue.
test.describe('Kanban cell colours', () => {
  test('paints the configured cells and labels them', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-heatmap');

    // Lanes high then low; in each lane the columns New then Fixed. High/Fixed
    // has no colour, so three cells are toned.
    await kanban.expectTonedCellCount(3);
    await kanban.expectTonedCell(0, 'Critical', '--rl-tag-red-bg');
    await kanban.expectCardInTonedCell(0, 'Login form validation');
    await kanban.expectTonedCell(1, 'Watch', '--rl-tag-amber-bg');
    await kanban.expectTonedCell(2, 'Good', '--rl-tag-green-bg');
  });
});
