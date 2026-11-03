import { test, expect } from './fixtures';
import { FormPage } from '../pages';

// BUG-NSGD6OE: following a link from one entity's edit form to another's keeps
// the same route, so the router reuses the form. An edit made just before the
// switch must be saved to the entity it was made on, and the form must then
// show the new entity.
test.describe('Edit form entity switch', () => {
  test('an edit stays on its entity after following a side-panel link', async ({ appPage, api }) => {
    // Equal descriptions: the old conflict merge treated B's value as A's base
    // and wrote A's edit onto B.
    const a = await api.createEntity('bugs', { properties: { title: 'Bug A', description: 'same' } });
    const b = await api.createEntity('bugs', { properties: { title: 'Bug B', description: 'same' } });
    await api.createRelation('bugs', a.id, 'blocks', b.id);

    const form = new FormPage(appPage);
    await form.navigateToEditForm('bug', a.id);
    await form.fillField('description', 'changed on A');
    await form.clickSidePanelEntity('Bug B');

    await form.expectAtFormUrl('bug', b.id);
    await form.expectFieldValue('title', 'Bug B');
    await form.expectFieldValue('description', 'same');
    await expect
      .poll(async () => (await api.getEntity('bugs', a.id)).properties.description)
      .toBe('changed on A');
    expect((await api.getEntity('bugs', b.id)).properties.description).toBe('same');
  });
});
