import { test, SEED } from './fixtures';
import { EntityPage } from '../pages';

// A display section in a custom view shows the enum labels from the schema,
// not the raw values. The fixture's "Classification" section renders `kind`
// (type task_kind) and `areas` (list of work_area): the property names differ
// from their enum type names, which is what the label lookup got wrong.
test.describe('View section enum labels', () => {
  test('shows schema labels for single and list enum fields', async ({ appPage, api }) => {
    await api.updateEntity('tasks', SEED.tasks.refactorAuth, {
      kind: 'external_obligation',
      areas: ['software_development', 'operations'],
    });

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('task', SEED.tasks.refactorAuth);

    await entity.expectSectionBadges('Classification', [
      'External obligation',
      'Software development',
      'Operations',
    ]);
  });
});
