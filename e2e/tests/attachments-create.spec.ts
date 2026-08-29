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
