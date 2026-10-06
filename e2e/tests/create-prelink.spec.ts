import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

// A section create pre-links the new entity to the page's entity. When the
// create form has no field for that relation, the form derives the peer's type
// from its id. The module type's id_prefix carries the trailing dash ("MOD-"),
// the documented spelling, and the task form has no `contains` field. The
// new task is the relation's target, so the edge must run module -> task.
test.describe('Section create pre-link', () => {
  test('links the new entity to a peer whose id_prefix ends in a dash', async ({ appPage, api }) => {
    const module = await api.createEntity('modules', {
      id: 'MOD-1',
      properties: { name: 'Billing' },
    });
    const entityPage = new EntityPage(appPage);

    await entityPage.navigateToEntity('module', module.id);
    await entityPage.clickSectionCreate('Tasks', 'Task');
    await entityPage.fillCreateDialogField('title', 'Prelinked task');
    await entityPage.submitCreateDialog();

    await entityPage.expectCreateDialogClosed();
    await entityPage.expectSectionRow('Tasks', 'Prelinked task');
    // Outgoing from the module: the edge was not written backwards.
    const linked = await api.listRelations('modules', module.id, 'contains');
    expect(linked.map((r) => r.id)).toEqual([expect.stringMatching(/^TASK-/)]);
  });
});
