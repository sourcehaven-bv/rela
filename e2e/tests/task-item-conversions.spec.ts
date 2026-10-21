import { test, expect } from './fixtures';
import { EntityPage, FormPage } from '../pages';

/**
 * Converting a task item into a linked entity (TKT-3IBLIP).
 *
 * The schema declares two recipes on `bug`, so pressing a convert button
 * opens a choice. The unit suites cover the editor transforms and the dialog
 * flow; these cases exist for what jsdom cannot decide: the button is
 * clickable in a real layout, pressing the dialog does not end the inline
 * body edit, and the edited body reaches the server.
 */
test.describe('Task item conversions', () => {
  let bugId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const bug = await api.createEntity('bugs', {
      properties: { title: 'Conversion source' },
      content: '- [ ] Write the migration\n- [ ] Keep this one\n',
    });
    bugId = bug.id;
  });

  test.afterEach(async ({ api }) => {
    if (!bugId) return;
    // Delete the tasks the conversions created, then the bug.
    const relations = (await api.getAllRelations('bugs', bugId).catch(() => ({}))) as Record<
      string,
      Array<{ id: string }>
    >;
    const tasks = Object.values(relations)
      .flat()
      .filter((r) => r.id.startsWith('TASK-'));
    for (const t of tasks) await api.deleteEntity('tasks', t.id).catch(() => {});
    await api.deleteEntity('bugs', bugId).catch(() => {});
  });

  test('the edit form removes a converted item and links the new task', async ({ appPage, api }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('bug', bugId!);
    await expect(form.editorTaskItemConvertButtons).toHaveCount(2);

    await form.editorTaskItems.first().hover();
    await form.editorTaskItemConvertButtons.first().click();
    await form.chooseTaskItemConversion('Convert to fix task');

    await expect(form.editorTaskItems).toHaveCount(1);
    await form.saveAndWaitForPatch('bugs', bugId!);

    await expect
      .poll(async () => (await api.getContent('bugs', bugId!)).trimEnd(), { timeout: 5000 })
      .toBe('- [ ] Keep this one');
    const relations = (await api.getAllRelations('bugs', bugId!)) as Record<
      string,
      Array<{ id: string }>
    >;
    const fixers = relations.fixedBy ?? [];
    expect(fixers).toHaveLength(1);
    const task = (await api.getEntity('tasks', fixers[0].id)) as {
      properties: { title?: string };
    };
    expect(task.properties.title).toBe('Write the migration');
  });

  test('the inline body editor stays open through the dialog and links the item', async ({
    appPage,
    api,
  }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', bugId!);
    await entity.bodyEditButton.click();
    await expect(entity.bodyEditor).toBeVisible();
    await expect(entity.bodyTaskItemConvertButtons).toHaveCount(2);

    await entity.bodyTaskItemConvertButtons.nth(1).click();
    await expect(entity.taskItemDialog).toBeVisible();
    await entity.chooseTaskItemConversion('Convert to linked task');

    await expect(entity.taskItemDialog).toHaveCount(0);
    await expect(entity.bodyEditor).toBeVisible();
    await expect(entity.bodyTaskItemConvertButtons).toHaveCount(1);
    await expect
      .poll(async () => (await api.getContent('bugs', bugId!)).trimEnd(), { timeout: 5000 })
      .toMatch(/^- \[ \] Write the migration\n- \[ \] `TASK-\d+`$/);
  });
});
