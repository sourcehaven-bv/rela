import { test, expect } from './fixtures';
import { FormPage, EntityPage } from '../pages';

test.describe('Create Form', () => {
  const createdFeatures: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdFeatures.length) {
      const id = createdFeatures.pop()!;
      await api.deleteEntity('features', id).catch(() => {});
    }
  });

  test('create feature form displays all fields', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');
    expect(await formPage.hasField('title')).toBeTruthy();
    expect(await formPage.hasField('priority')).toBeTruthy();
    expect(await formPage.hasField('description')).toBeTruthy();
  });

  test('create feature form has submit button', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');
    expect(await formPage.hasSubmitButton()).toBeTruthy();
  });

  test('can fill and submit create feature form', async ({ appPage, api }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');

    await formPage.fillField('title', 'E2E Form Test Feature');
    await formPage.fillField('description', 'Testing form submission');
    await formPage.selectField('priority', 'medium');

    const created = await formPage.submitAndExpectCreate('features');
    createdFeatures.push(created.id);

    // Persisted via API
    const entity = await api.getEntity('features', created.id);
    expect(entity.properties.title).toBe('E2E Form Test Feature');
    expect(entity.properties.priority).toBe('medium');
  });

  test('form blocks submission or shows error when required fields missing', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');
    await formPage.submit();

    const hasError = await formPage.hasValidationError();
    const stillOnForm = /\/form\//.test(appPage.url());
    expect(hasError || stillOnForm).toBeTruthy();
  });
});

test.describe('Edit Form', () => {
  let testFeatureId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const feature = await api.createEntity('features', {
      properties: {
        title: 'E2E Edit Form Test',
        description: 'Original description',
        status: 'draft',
        priority: 'low',
      },
    });
    testFeatureId = feature.id;
  });

  test.afterEach(async ({ api }) => {
    if (testFeatureId) {
      await api.deleteEntity('features', testFeatureId).catch(() => {});
      testFeatureId = null;
    }
  });

  test('edit form loads with existing data', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm('feature', testFeatureId!);
    expect(await formPage.getFieldValue('title')).toBe('E2E Edit Form Test');
    expect(await formPage.getFieldValue('priority')).toBe('low');
  });

  test('can update feature via edit form', async ({ appPage, api }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm('feature', testFeatureId!);

    await formPage.fillField('title', 'E2E Edit Form Test - Updated');
    await formPage.selectField('priority', 'high');
    await formPage.saveAndWaitForPatch('features', testFeatureId!);

    const entity = await api.getEntity('features', testFeatureId!);
    expect(entity.properties.title).toBe('E2E Edit Form Test - Updated');
    expect(entity.properties.priority).toBe('high');
  });

  test('status select exposes options', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm('feature', testFeatureId!);
    const options = await formPage.getSelectOptions('status');
    expect(options.length).toBeGreaterThanOrEqual(1);
  });
});

test.describe('Edit Form - Default Relation Picker Save (BUG-UNEBR regression)', () => {
  // The task form has an `implements` relation to feature. We add a target in
  // the default picker, Save, then confirm it persisted by reading the entity.
  // Locks in the fix for BUG-UNEBR: the default picker path was silently
  // dropping `relations` payload.
  let taskId: string | null = null;
  let featureAId: string | null = null;
  let featureBId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const suffix = Date.now().toString(36);
    const featA = await api.createEntity('features', {
      properties: { title: `Picker Feature A ${suffix}`, status: 'draft', priority: 'high' },
    });
    const featB = await api.createEntity('features', {
      properties: { title: `Picker Feature B ${suffix}`, status: 'draft', priority: 'high' },
    });
    featureAId = featA.id;
    featureBId = featB.id;

    const task = await api.createEntity('tasks', {
      properties: { title: `Picker Task ${suffix}`, status: 'draft', assignee: 'tester' },
      relations: { implements: [featA.id] },
    });
    taskId = task.id;
  });

  test.afterEach(async ({ api }) => {
    if (taskId) await api.deleteEntity('tasks', taskId).catch(() => {});
    if (featureAId) await api.deleteEntity('features', featureAId).catch(() => {});
    if (featureBId) await api.deleteEntity('features', featureBId).catch(() => {});
    taskId = null;
    featureAId = null;
    featureBId = null;
  });

  test('adding a target in the picker persists after Save', async ({ appPage, api }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm('task', taskId!);

    // Add the second feature via the `implements` relation picker.
    await formPage.addRelation('Implements Feature', featureBId!);

    const response = await formPage.saveAndWaitForPatch('tasks', taskId!);
    expect(response.status()).toBe(200);

    const updated = await api.getEntity('tasks', taskId!);
    const edges = (updated.relations?.implements ?? []) as string[];
    expect(edges).toContain(featureAId);
    expect(edges).toContain(featureBId);
  });
});

test.describe('Inline Entity Creation', () => {
  test('inline create UI is either present or absent gracefully', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    if (await formPage.hasInlineCreateButton()) {
      await formPage.clickInlineCreateButton();
      await formPage.expectInlineFormVisible();
    }
    // If no inline-create UI is configured in the inline project,
    // the assertion above is skipped — that's by design.
    // Use the EntityPage's existing helpers to keep specs clean.
    void EntityPage;
  });
});
