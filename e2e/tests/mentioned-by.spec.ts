import { test, expect, type ApiHelpers } from './fixtures';
import { EntityPage } from '../pages';

// The "Mentioned by" card on an entity page lists the entities whose body
// names it as an `id` code span (TKT-Q6AKUN): the reverse of the in-app
// links entity-refs.spec.ts covers.
test.describe('Mentioned by', () => {
  const created: string[] = [];

  test.afterEach(async ({ api }) => {
    for (const id of created.splice(0)) {
      await api.deleteEntity('features', id).catch(() => {});
    }
  });

  async function feature(api: ApiHelpers, title: string, content = ''): Promise<string> {
    const e = await api.createEntity('features', {
      properties: { title, description: title, status: 'draft', priority: 'medium' },
      content,
    });
    created.push(e.id);
    return e.id;
  }

  test('lists the mentioning entity and links to it', async ({ api, appPage }) => {
    const target = await feature(api, 'Mention Target');
    const source = await feature(api, 'Mention Source', `depends on \`${target}\``);

    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', target);
    await expect(entity.mentionedByLink('feature', source)).toContainText('Mention Source');

    await entity.clickMentionedBy('feature', source);
    await entity.expectHeadingText('Mention Source');
  });

  test('is absent when nothing mentions the entity', async ({ api, appPage }) => {
    const lonely = await feature(api, 'Unmentioned');
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', lonely);
    await entity.expectHeadingText('Unmentioned');
    await expect(entity.mentionedByPanel).toHaveCount(0);
  });
});
