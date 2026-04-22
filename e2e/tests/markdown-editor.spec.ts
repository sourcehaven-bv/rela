import { test, expect } from './fixtures';
import { FormPage } from '../pages';

test.describe('Markdown Body Editor', () => {
  const createdFeatures: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdFeatures.length) {
      const id = createdFeatures.pop()!;
      await api.deleteEntity('features', id).catch(() => {});
    }
  });

  test('create form shows the Content body field', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectContentFieldVisible();
    await form.expectContentLabelHasText('Content');
  });

  test('markdown editor renders with toolbar', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');
    await form.expectMarkdownEditorReady();
  });

  test('can fill body content and submit form', async ({ appPage, api }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('feature');

    await form.fillField('title', 'E2E Body Test Feature');
    await form.selectField('priority', 'low');
    await form.expectMarkdownEditorReady();
    await form.typeMarkdownBody('# Test Content\n\nThis is body content.');

    const created = await form.submitAndExpectCreate('features');
    createdFeatures.push(created.id);

    const entity = await api.getEntity('features', created.id);
    expect(entity.properties.title).toBe('E2E Body Test Feature');
  });
});
