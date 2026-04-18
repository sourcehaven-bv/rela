import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

test.describe('Create entity redirect', () => {
  test('detail page loads after create without error', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    const entityPage = new EntityPage(appPage);

    await formPage.navigateToCreateForm('feature');
    await formPage.fillFields({ title: 'Redirect Test Feature' });
    await formPage.selectFields({ status: 'draft', priority: 'high' });
    await formPage.submitAndExpectCreate('features');

    await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-\d+/);
    await entityPage.expectHeadingText('Redirect Test Feature');
    await entityPage.expectNoErrorState();
  });

  test('detail page loads after rapid create (stress)', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    const entityPage = new EntityPage(appPage);

    for (let i = 1; i <= 3; i++) {
      const title = `Stress Test Feature ${i}`;
      await formPage.navigateToCreateForm('feature');
      await formPage.fillFields({ title });
      await formPage.selectFields({ status: 'draft' });
      await formPage.submitAndExpectCreate('features');

      await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-\d+/);
      await entityPage.expectHeadingText(title);
      await entityPage.expectNoErrorState();
    }
  });
});
