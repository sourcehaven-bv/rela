import { test, expect } from './fixtures';
import { FormPage, ListPage } from '../pages';

// Attaching a file while CREATING an entity (TKT-7K3BJF).
//
// This is the real integration proof for the feature: an attachment cannot be
// written before the entity row exists (every store backend returns
// ErrNotFound) and no id can be reserved ahead of the create, so the SPA
// stages the picked bytes and uploads them once the server returns an id.
// Only an end-to-end run exercises both phases against a real store.
test.describe('Attachments on entity create', () => {
  test('stages a file in the create form and attaches it after save', async ({ appPage, api }) => {
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();

    await formPage.fillField('title', 'Bug with a screenshot');

    // The control must exist at all — before this ticket the create form
    // rendered a dead "Attachment editing unavailable" note here.
    expect(await formPage.canAddFile('screenshot')).toBe(true);

    await formPage.attachFile('screenshot', 'shot.txt', 'pretend png bytes');

    // Staged, not uploaded: the file is listed, but no entity exists yet.
    expect(await formPage.attachedFileNames('screenshot')).toEqual(['shot.txt']);

    const created = await formPage.submitAndExpectCreateWithUploads('bugs', 1);

    // The server is the arbiter: the attachment must be readable back on the
    // entity, and its property stamped.
    const entity = await api.getEntity('bugs', created.id);
    const files = entity._attachments?.screenshot ?? [];
    expect(files).toHaveLength(1);
    expect(files[0].filename).toBe('shot.txt');
    expect(entity.properties.screenshot).toBeTruthy();
  });

  test('a staged file removed before save is never uploaded', async ({ appPage, api }) => {
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();
    await formPage.fillField('title', 'Bug without a screenshot');

    await formPage.attachFile('screenshot', 'oops.txt', 'unwanted');
    expect(await formPage.attachedFileNames('screenshot')).toEqual(['oops.txt']);

    await formPage.removeAttachedFile('screenshot');
    expect(await formPage.attachedFileNames('screenshot')).toEqual([]);

    const created = await formPage.submitAndExpectCreate('bugs');

    const entity = await api.getEntity('bugs', created.id);
    expect(entity._attachments?.screenshot ?? []).toHaveLength(0);
  });

  test('stages several files on a multi-cap property and uploads them all', async ({
    appPage,
    api,
  }) => {
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();
    await formPage.fillField('title', 'Bug with evidence');

    // `evidence` is max: 3 in the fixture schema.
    await formPage.attachFile('evidence', 'one.txt', 'first');
    await formPage.attachFile('evidence', 'two.txt', 'second');
    expect(await formPage.attachedFileNames('evidence')).toEqual(['one.txt', 'two.txt']);

    const created = await formPage.submitAndExpectCreateWithUploads('bugs', 2);

    const entity = await api.getEntity('bugs', created.id);
    const names = (entity._attachments?.evidence ?? []).map((f) => f.filename).sort();
    expect(names).toEqual(['one.txt', 'two.txt']);
  });

  test('hides the add control once a multi-cap property is full', async ({ appPage }) => {
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();

    // Staged files count toward the cap exactly as uploaded ones do in edit
    // mode, so the third file fills `evidence` (max: 3).
    await formPage.attachFile('evidence', 'a.txt', 'a');
    await formPage.attachFile('evidence', 'b.txt', 'b');
    expect(await formPage.canAddFile('evidence')).toBe(true);

    await formPage.attachFile('evidence', 'c.txt', 'c');
    expect(await formPage.canAddFile('evidence')).toBe(false);
  });

  // The accepted tradeoff of the two-phase design: create can succeed and the
  // upload fail on its own. The whole feature rests on that being loud, so it
  // is worth proving against a real server rejection rather than a mock.
  test('an oversize file fails loudly after the entity is created', async ({ appPage, api }) => {
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();
    await formPage.fillField('title', 'Bug with an oversize file');

    // The fixture caps attachments at 1 KiB; 4 KiB is a real 413 from the
    // server's ingress guard, not a simulated one.
    await formPage.attachFile('screenshot', 'huge.txt', 'x'.repeat(4096));

    const created = await formPage.submitAndExpectCreate('bugs');

    // The entity exists and the user is on it — losing the entity would be
    // worse than losing the file.
    await expect(appPage).toHaveURL(new RegExp(`/entity/bug/${created.id}`));

    // The failure is reported, names the file, and says where to fix it.
    const toast = await formPage.errorToastText();
    expect(toast).toContain('huge.txt');
    expect(toast).toContain('too large');
    expect(toast).toContain('Re-attach on the entity');

    // Nothing was attached, and no half-written property was stamped.
    const entity = await api.getEntity('bugs', created.id);
    expect(entity._attachments?.screenshot ?? []).toHaveLength(0);
    expect(entity.properties.screenshot ?? '').toBe('');
  });

  // BUG-L1DHC5: staged files live outside `formData`, which is where the
  // required check looks — so before the fix a required file property could
  // never be satisfied and Create did nothing at all.
  test.describe('a required file property', () => {
    test('blocks Create until a file is staged, then goes through', async ({ appPage, api }) => {
      const formPage = new FormPage(appPage);
      await formPage.navigateToCreateForm('bug_signoff');
      await formPage.fillField('title', 'Needs sign-off');

      // No file yet: submitting must not create anything, and must say why.
      await formPage.submitExpectingNoCreate('signoffs');
      await expect(appPage).toHaveURL(/\/form\/bug_signoff/);
      expect(await formPage.fieldErrorText('document')).toContain('required');

      // Staging one satisfies it — the whole point of the fix.
      await formPage.attachFile('document', 'signed.txt', 'approved');
      const created = await formPage.submitAndExpectCreateWithUploads('signoffs', 1);

      const entity = await api.getEntity('signoffs', created.id);
      expect((entity._attachments?.document ?? []).map((f) => f.filename)).toEqual(['signed.txt']);
    });
  });

  test('creating without touching a file property is unchanged', async ({ appPage, api }) => {
    // The common case by far: it must not gain a request or change behaviour.
    const listPage = new ListPage(appPage);
    const formPage = new FormPage(appPage);

    await listPage.navigateToList('bugs');
    await listPage.clickCreateButton();
    await formPage.fillField('title', 'Plain bug');

    const created = await formPage.submitAndExpectCreate('bugs');

    const entity = await api.getEntity('bugs', created.id);
    expect(entity.properties.title).toBe('Plain bug');
    expect(entity._attachments?.screenshot ?? []).toHaveLength(0);
  });
});
