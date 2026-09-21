// Duplicating an entity from the detail page (TKT-Z8K2FS).
//
// The assertions here are deliberately about the RESULT of the copy, read back
// through the API, rather than about the dialog having been clicked. An
// earlier inline-create spec asserted only on loose selectors and turned out to
// be vacuous (see pages/form.page.ts), so every step that claims an edge was
// carried proves it by reading the new entity's relations.

import { test, expect } from './fixtures';
import { EntityPage } from '../pages';
import { SEED } from './fixtures';

test.describe('Duplicate entity', () => {
  const created: string[] = [];

  test.afterEach(async ({ api }) => {
    while (created.length) {
      const id = created.pop()!;
      await api.deleteEntity('features', id).catch(() => {});
    }
  });

  test('copies properties, content and the chosen relations', async ({ appPage, api }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.authentication);

    // AC1: the affordance is offered for a type the principal may create.
    await expect(entity.duplicateButton).toBeVisible();
    await entity.openDuplicate();

    // AC2/AC3: FEAT-001 has `tagged` -> FEAT-002 and `blocks` -> FEAT-003,
    // both outgoing, so both are listed and both default to checked.
    await expect(entity.duplicateChoiceCheckbox('tagged')).toBeChecked();
    await expect(entity.duplicateChoiceCheckbox('blocks')).toBeChecked();
    await expect(entity.duplicateChoiceCount('tagged')).toHaveText('1');

    // AC7: uncheck one type; none of its edges may reach the copy.
    await entity.duplicateChoiceCheckbox('blocks').uncheck();
    await entity.continueDuplicate();

    // AC5/AC8: the form is prefilled from the source, and the title is
    // editable BEFORE the copy exists — which is the whole reason this flow
    // goes through a form rather than writing server-side.
    const title = entity.duplicateField('title');
    await expect(title).toHaveValue('User Authentication');
    await title.fill('Duplicated Authentication');
    await entity.submitDuplicateForm();

    // AC6: landing on the copy proves the create succeeded and the SPA
    // navigated to it.
    await expect(entity.heading).toHaveText('Duplicated Authentication', { timeout: 10_000 });
    const newId = appPage.url().split('/').pop()!;
    created.push(newId);

    const copy = await api.getEntity('features', newId);
    expect(copy.properties.title).toBe('Duplicated Authentication');
    // AC9: a property the user never touched still carries.
    expect(copy.properties.priority).toBe('high');

    // AC6/AC7 read back from the graph, not from the dialog.
    const rels = await api.getAllRelations('features', newId);
    expect(rels.tagged?.map((e) => e.id)).toEqual([SEED.features.dashboardAnalytics]);
    expect(rels.blocks).toBeUndefined();
  });

  test('carries an incoming relation under its inverse key', async ({ appPage, api }) => {
    // The create body accepts inverse-named keys and resolves the direction
    // server-side, so an incoming edge rides the SAME batched create as an
    // outgoing one. This asserts that end to end, because the plan originally
    // assumed the opposite and would have built a post-create write path.
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', SEED.features.exportData);

    await entity.openDuplicate();
    // FEAT-001 blocks FEAT-003, so FEAT-003 sees it as incoming `blockedBy`,
    // unchecked by default (AC3).
    await expect(entity.duplicateChoiceCheckbox('blockedBy')).not.toBeChecked();
    await entity.duplicateChoiceCheckbox('blockedBy').check();
    await entity.continueDuplicate();

    await entity.duplicateField('title').fill('Export With Blocker');
    await entity.submitDuplicateForm();

    await expect(entity.heading).toHaveText('Export With Blocker', { timeout: 10_000 });
    const newId = appPage.url().split('/').pop()!;
    created.push(newId);

    const rels = await api.getAllRelations('features', newId);
    expect(rels.blockedBy?.map((e) => e.id)).toEqual([SEED.features.authentication]);
  });
});
