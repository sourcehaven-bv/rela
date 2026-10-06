import { test, expect } from './fixtures';
import { FormPage } from '../pages';

// BUG-ZD4PIN: a form create does not add a status the type does not declare.
test.describe('Create form status default', () => {
  test('a type without a status property gets no status', async ({ appPage, api }) => {
    const formPage = new FormPage(appPage);

    await formPage.navigateToCreateForm('decision');
    await formPage.fillFields({ title: 'No status decision' });
    const created = await formPage.submitAndExpectCreate('decisions');

    const stored = await api.getEntity('decisions', created.id);
    expect(stored.properties.status).toBeUndefined();
  });
});
