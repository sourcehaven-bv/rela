import { test } from './fixtures';
import { KanbanPage } from '../pages';

// TKT-QYB9N7: a column folds to a rail with its title and count, the choice
// survives a reload, and `collapsed: true` in config sets the default.
test.describe('Kanban column collapse', () => {
  test('collapses and expands a column on a plain board, across a reload', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-board');

    await kanban.collapseColumn('New');
    await kanban.expectColumnCollapsed('New', 1);

    await appPage.reload();
    await kanban.waitForSpinnerToDisappear();
    await kanban.expectColumnCollapsed('New', 1);

    await kanban.expandColumn('New');
    await kanban.expectCardInColumn('Login form validation', 'New');
  });

  test('a card dropped on a collapsed column stays where it was', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-board');

    await kanban.collapseColumn('Fixed');
    await kanban.dragCardToCollapsedColumn('Login form validation', 'Fixed');

    await kanban.expectCardInColumn('Login form validation', 'New');
    await kanban.expectColumnCollapsed('Fixed', 0);
  });

  test('starts a config-collapsed column folded on a swimlane board', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-lanes');

    await kanban.expectColumnCollapsed('In Progress', 1);

    await kanban.expandColumn('In Progress');
    await kanban.expectColumnExpanded('In Progress');

    await appPage.reload();
    await kanban.waitForSpinnerToDisappear();
    await kanban.expectColumnExpanded('In Progress');

    await kanban.collapseColumn('Fixed');
    await kanban.expectColumnCollapsed('Fixed', 0);
  });
});
