import { test, expect } from './fixtures';
import { KanbanPage } from '../pages';

// TKT-IP4273S: a title typed under a column, or under a swimlane cell, makes a
// card with that column's (and lane's) value, through the ordinary create API.
test.describe('Kanban quick-add', () => {
  test('adds a card to a column and keeps it after a reload', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-quick');

    await kanban.quickAdd('Typo in footer', 'Fixed');
    await kanban.expectCardInColumn('Typo in footer', 'Fixed');
    await kanban.expectQuickAddReady('Fixed');

    await appPage.reload();
    await kanban.waitForSpinnerToDisappear();
    await kanban.expectCardInColumn('Typo in footer', 'Fixed');
  });

  test('sets the swimlane value from the cell', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-quick-lanes');

    await kanban.quickAdd('Crash on save', 'New', 'Critical');
    await kanban.expectCardInCell('Crash on save', 'New', 'Critical');

    await appPage.reload();
    await kanban.waitForSpinnerToDisappear();
    await kanban.expectCardInCell('Crash on save', 'New', 'Critical');
  });

  test('is absent on a board without quick_add', async ({ appPage }) => {
    const kanban = new KanbanPage(appPage);
    await kanban.navigateToKanban('bug-board');
    await expect(kanban.columns.first()).toBeVisible();
    await expect(kanban.quickAddInputs).toHaveCount(0);
  });
});
