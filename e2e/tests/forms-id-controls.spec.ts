import { test, expect } from './fixtures';
import { FormPage } from '../pages';

// TKT-E7NNM: prefix picker for multi-prefix types + manual ID field for
// manual-ID types. These specs drive the /form/:type routes for the four
// fixture entity types that span the id_type × id_prefix(es) matrix:
// tag (manual, no prefix), decision (short, multi prefix),
// module (manual, single prefix), specification (manual, multi prefix).

async function openCreateForm(appPage: import('@playwright/test').Page, formId: string) {
  const formPage = new FormPage(appPage);
  await formPage.navigateTo(`/form/${formId}`);
  await formPage.waitForSpinnerToDisappear();
  await expect(formPage.form).toBeVisible();
  return formPage;
}

async function submitAndWaitForCreate(
  appPage: import('@playwright/test').Page,
  formPage: FormPage,
  plural: string,
): Promise<import('@playwright/test').Response> {
  const [resp] = await Promise.all([
    appPage.waitForResponse(
      (r) => r.url().includes(`/api/v1/${plural}`) && r.request().method() === 'POST',
    ),
    formPage.submitButton.first().click(),
  ]);
  return resp;
}

test.describe('Manual-ID Create Form', () => {
  const createdTags: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdTags.length) {
      const id = createdTags.pop()!;
      await api.deleteEntity('tags', id).catch(() => {});
    }
  });

  test('renders ID input and creates tag with user-supplied ID', async ({ appPage, api }) => {
    const formPage = await openCreateForm(appPage, 'tag');

    await formPage.expectIdInputVisible();

    const tagId = `e2e-tag-${Date.now()}`;
    await formPage.fillId(tagId);
    await formPage.fillField('name', 'E2E Manual Tag');

    const resp = await submitAndWaitForCreate(appPage, formPage, 'tags');
    expect(resp.status()).toBe(201);

    const fetched = await api.getEntity('tags', tagId);
    expect(fetched.id).toBe(tagId);
    createdTags.push(tagId);
  });
});

test.describe('Multi-Prefix Create Form', () => {
  const createdDecisions: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdDecisions.length) {
      const id = createdDecisions.pop()!;
      await api.deleteEntity('decisions', id).catch(() => {});
    }
  });

  test('shows prefix picker and creates entity with chosen prefix', async ({ appPage }) => {
    const formPage = await openCreateForm(appPage, 'decision');

    await formPage.expectPrefixSelectVisible();
    const options = await formPage.getPrefixOptions();
    expect(options).toContain('DEC-');
    expect(options).toContain('ADR-');

    await formPage.selectPrefix('ADR-');
    await formPage.fillField('title', 'E2E Multi Prefix Decision');

    const resp = await submitAndWaitForCreate(appPage, formPage, 'decisions');
    expect(resp.status()).toBe(201);
    const body = (await resp.json()) as { id: string };
    expect(body.id.startsWith('ADR-')).toBe(true);
    createdDecisions.push(body.id);
  });

  test('does not show prefix picker for single-prefix feature form', async ({ appPage }) => {
    const formPage = new FormPage(appPage);
    await formPage.navigateToCreateForm('feature');

    await formPage.expectPrefixSelectHidden();
  });

  test('edit mode does not show prefix picker', async ({ api, appPage }) => {
    const created = await api.createEntity('decisions', {
      prefix: 'DEC-',
      properties: { title: 'E2E Edit Mode No Picker' },
    });
    try {
      const formPage = new FormPage(appPage);
      await formPage.navigateTo(`/form/decision/${created.id}`);
      await formPage.waitForSpinnerToDisappear();

      await formPage.expectPrefixSelectHidden();
    } finally {
      await api.deleteEntity('decisions', created.id).catch(() => {});
    }
  });
});

test.describe('Manual-ID with Prefix Enforcement', () => {
  const createdModules: string[] = [];
  const createdSpecs: string[] = [];

  test.afterEach(async ({ api }) => {
    while (createdModules.length) {
      await api.deleteEntity('modules', createdModules.pop()!).catch(() => {});
    }
    while (createdSpecs.length) {
      await api.deleteEntity('specifications', createdSpecs.pop()!).catch(() => {});
    }
  });

  test('rejects bare ID when single prefix is declared', async ({ appPage }) => {
    const formPage = await openCreateForm(appPage, 'module');

    await formPage.expectIdInputVisible();
    await formPage.fillId('no-prefix-here');
    await formPage.fillField('name', 'E2E Bare ID Reject');

    const resp = await submitAndWaitForCreate(appPage, formPage, 'modules');
    expect(resp.status()).toBe(422);
    const body = await resp.text();
    expect(body).toMatch(/must start with.*MOD-/);
    expect(appPage.url()).toContain('/form/module');
  });

  test('accepts ID with declared single prefix', async ({ appPage, api }) => {
    const formPage = await openCreateForm(appPage, 'module');

    const moduleId = `MOD-e2e-${Date.now()}`;
    await formPage.fillId(moduleId);
    await formPage.fillField('name', 'E2E Prefixed Module');

    const resp = await submitAndWaitForCreate(appPage, formPage, 'modules');
    expect(resp.status()).toBe(201);

    const fetched = await api.getEntity('modules', moduleId);
    expect(fetched.id).toBe(moduleId);
    createdModules.push(moduleId);
  });

  test('accepts any of the multiple declared prefixes', async ({ appPage, api }) => {
    const formPage = await openCreateForm(appPage, 'specification');

    const specId = `SPEC-e2e-${Date.now()}`;
    await formPage.fillId(specId);
    await formPage.fillField('name', 'E2E Multi Prefix Manual');

    const resp = await submitAndWaitForCreate(appPage, formPage, 'specifications');
    expect(resp.status()).toBe(201);

    const fetched = await api.getEntity('specifications', specId);
    expect(fetched.id).toBe(specId);
    createdSpecs.push(specId);
  });
});

test.describe('Create Form Error Surfacing', () => {
  test('surfaces duplicate-ID server error message', async ({ appPage, api }) => {
    const tagId = `e2e-dup-${Date.now()}`;
    const existing = await api.createEntity('tags', {
      id: tagId,
      properties: { name: 'Existing' },
    });
    try {
      const formPage = await openCreateForm(appPage, 'tag');

      await formPage.fillId(tagId);
      await formPage.fillField('name', 'Duplicate attempt');

      const resp = await submitAndWaitForCreate(appPage, formPage, 'tags');
      expect(resp.status()).toBeGreaterThanOrEqual(400);
      const body = await resp.text();
      expect(body).toContain('already exists');
      expect(body).toContain(tagId);
    } finally {
      await api.deleteEntity('tags', existing.id).catch(() => {});
    }
  });
});
