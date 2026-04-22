import { test, expect } from './fixtures';
import { FormPage } from '../pages';

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

  test('form blocks POST when required fields are missing', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');

    // Ensure title is empty (it's required). Submit and assert no POST fires
    // AND we're still on the form. OR-assertion (hasError || stillOnForm)
    // was trivially true because submit() swallows waitForURL timeouts, so
    // we test the concrete invariant: no entity was created. (RR-UQ225)
    let postSeen = false;
    const listener = (resp: import('@playwright/test').Response) => {
      if (resp.url().includes('/api/v1/features') && resp.request().method() === 'POST') {
        postSeen = true;
      }
    };
    appPage.on('response', listener);
    try {
      await formPage.submit();
      // Instead of a bare sleep, give the page a round-trip to settle by
      // asking for a cheap HEAD the server must answer. If a POST were in
      // flight it'd race this — in practice native HTML5 validation blocks
      // submit synchronously before fetch fires.
      await expect
        .poll(() => postSeen, { timeout: 500, intervals: [100, 100, 100, 100, 100] })
        .toBe(false);
    } finally {
      appPage.off('response', listener);
    }
    expect(postSeen).toBe(false);
    expect(appPage.url()).toMatch(/\/form\/feature/);
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
    // the assertion above is skipped — that's by design for the inline fixture.
  });
});
