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
      relations: { implements: { data: [{ type: 'feature', id: featA.id }] } },
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

    // After TKT-E6094, edit forms autosave on debounce. Arm the
    // PATCH waiter BEFORE the picker action so the response can't
    // land before we start listening.
    const patchPromise = appPage.waitForResponse(
      (r) => r.url().includes(`/api/v1/tasks/${taskId!}`) && r.request().method() === 'PATCH',
      { timeout: 5000 },
    );

    // Add the second feature via the `implements` relation picker.
    await formPage.addRelation('Implements Feature', featureBId!);

    const response = await patchPromise;
    expect(response.status()).toBe(200);

    const updated = await api.getEntity('tasks', taskId!);
    const edges = (updated.relations?.implements ?? []) as string[];
    expect(edges).toContain(featureAId);
    expect(edges).toContain(featureBId);
  });
});

// Cancel/Back must land inside the app (BUG-ZE4354).
//
// `router.back()` is a browser-history operation with no route target, so on a
// form opened COLD — the exact shape below, a direct navigation with no prior
// in-app entry — it walked out of the SPA entirely. The existing specs never
// caught it because they all reach forms via an in-app link, which leaves a
// history entry to go back to.
test.describe('Form cancel navigation', () => {
  test('cancel on a directly-opened form stays in the app', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');

    await formPage.cancel();

    // The load-bearing assertions: we left the form, we are on a real in-app
    // route, and the SPA is still mounted. Before the fix this landed on the
    // browser's pre-app page (about:blank in a fresh context).
    await expect(appPage).not.toHaveURL(/\/form\//);
    await expect(appPage).toHaveURL(/\/(list\/[a-z_]+|$)/);
    expect(appPage.url()).not.toContain('about:blank');
    await expect(formPage.appChrome).toBeVisible();
  });

  test('cancel still goes back when there is in-app history', async ({ appPage }) => {
    // No regression for the common path.
    const formPage = new FormPage(appPage);
    await formPage.navigateToList('features');
    await formPage.navigateToCreateForm('feature');

    await formPage.cancel();

    await expect(appPage).toHaveURL(/\/list\/features/);
  });
});

// Inline entity creation from a relation field (TKT-OMUD56).
//
// The affordance is derived, not configured: it appears for a target type when
// the principal may create it AND a form resolves for it. In this fixture
// `feature` has a form and `tag` does not, so the task form's
// implements -> feature relation offers it and the feature form's
// tagged -> tag relation does not.
//
// The load-bearing test here is draft survival: the whole point of creating in
// a modal rather than navigating is that the host form's in-progress input
// stays put. That is an interaction between two mounted DynamicForms, which
// unit tests with a mocked router cannot reproduce.
test.describe('Inline Entity Creation', () => {
  test('offers "+ New" for a target type that has a form', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    const picker = formPage.relationPickerByLabel('Implements Feature');
    await formPage.openRelationPicker(picker);

    await expect(
      formPage.inlineCreateButtons(picker).filter({ hasText: '+ New Feature' }),
    ).toBeVisible();
  });

  test('offers "+ New" from the cards widget too', async ({ appPage, api }) => {
    // RelationCards had no inline-create affordance at all before this. It
    // needs its own, and the created entity must land in the LINK step (not be
    // linked outright) so required edge properties can still be filled.
    //
    // Uses the EDIT form because RelationCards requires an entityId; on a
    // create form the same relation falls back to a picker.
    //
    // (Every entity type in this fixture has a form, so the "no form => no
    // affordance" arm has no fixture to exercise here; it is covered by
    // TestInlineCreate_RequiresAForm on the server and by the
    // useInlineCreate/widget unit tests.)
    const feature = await api.createEntity('features', {
      properties: { title: 'Inline create from cards', status: 'draft', priority: 'low' },
    });

    const formPage = new FormPage(appPage);
    await formPage.navigateToEditForm('feature', feature.id);

    const tagged = formPage.relationCardsByLabel('tagged');
    await formPage.openCardsAddPanel(tagged);
    // `tagged` is feature -> feature in this fixture.
    await formPage.clickInlineCreate('Feature', tagged);
    await formPage.expectInlineFormVisible();

    await formPage.fillInlineField('title', 'Created from cards');
    await formPage.submitInlineForm();
    await expect(formPage.inlineCreateModal).toBeHidden();

    // Selected for linking, with the edge-meta form shown — not yet linked,
    // so the relation's `added_by` / `added_date` can still be filled.
    await expect(formPage.cardsPendingLink(tagged)).toBeVisible();
    await expect(formPage.cardsCreatedNotice(tagged)).toContainText('Created from cards');

    await api.deleteEntity('features', feature.id).catch(() => {});
  });

  test('opens a real configured form, not a metamodel dump', async ({ appPage }) => {
    // The removed modal rendered every metamodel property with an ad-hoc
    // widget dispatch. The nested form must render the CONFIGURED form: the
    // feature form declares title/status/priority/description and a body.
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    const picker = formPage.relationPickerByLabel('Implements Feature');
    await formPage.openRelationPicker(picker);
    await formPage.clickInlineCreate('Feature', picker);

    await formPage.expectInlineFormVisible();
    await expect(formPage.inlineField('title')).toBeVisible();
    await expect(formPage.inlineField('status')).toBeVisible();
    await expect(formPage.inlineField('priority')).toBeVisible();
  });

  test('creating inline links the new entity and preserves the host draft', async ({
    appPage,
    api,
  }) => {
    // AC5 + AC6 together, which is the feature in one test: the host form's
    // typed input must survive, and the created entity must end up linked.
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    await formPage.fillField('title', 'Host task draft');
    await formPage.fillField('assignee', 'jeroen');

    const picker = formPage.relationPickerByLabel('Implements Feature');
    await formPage.openRelationPicker(picker);
    await formPage.clickInlineCreate('Feature', picker);
    await formPage.expectInlineFormVisible();

    await formPage.fillInlineField('title', 'Created inline');
    await formPage.submitInlineForm();
    await expect(formPage.inlineCreateModal).toBeHidden();

    // The host draft is intact — no navigation happened.
    await expect(formPage.field('title')).toHaveValue('Host task draft');
    await expect(formPage.field('assignee')).toHaveValue('jeroen');
    // ...and the new feature is selected in the picker.
    await expect(formPage.pickerSelections(picker)).toContainText('Created inline');

    // Saving the host persists the edge.
    const created = await formPage.submitAndExpectCreate('tasks');
    const task = await api.getEntity('tasks', created.id);
    const edges = (task.relations?.implements ?? []) as string[];
    expect(edges).toHaveLength(1);

    await api.deleteEntity('tasks', created.id).catch(() => {});
    await api.deleteEntity('features', edges[0]).catch(() => {});
  });

  test('cancelling leaves the host draft untouched and creates nothing', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    await formPage.fillField('title', 'Untouched draft');

    const picker = formPage.relationPickerByLabel('Implements Feature');
    await formPage.openRelationPicker(picker);
    await formPage.clickInlineCreate('Feature', picker);
    await formPage.expectInlineFormVisible();

    await formPage.cancelInlineForm();

    await expect(formPage.inlineCreateModal).toBeHidden();
    await expect(formPage.field('title')).toHaveValue('Untouched draft');
    await expect(formPage.pickerSelections(picker)).toHaveCount(0);
  });

  test('a nested form does not offer inline create in turn', async ({ appPage }) => {
    // AC10, the structural depth cap. The feature form has relation widgets of
    // its own; inside the modal none of them may offer a further "+ New",
    // because a modal-on-modal has no defined Escape recipient.
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('task');

    const picker = formPage.relationPickerByLabel('Implements Feature');
    await formPage.openRelationPicker(picker);
    await formPage.clickInlineCreate('Feature', picker);
    await formPage.expectInlineFormVisible();

    await expect(formPage.inlineCreateButtons(formPage.inlineCreateModal)).toHaveCount(0);
  });
});
