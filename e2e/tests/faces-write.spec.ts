import { facedTest as test, expect, FACED_SEED, FACE, WORLD } from './faced-fixtures';
import { CommentsPage, FacesPage, FormPage, ListPage } from '../pages';

/**
 * Writing to one face of a faced type (TKT-WCMW47). Runs as the editor, whose
 * create/update/delete grant names `policy@draft` only.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;

test.describe('Faces: write', () => {
  test('editing the draft face leaves the published face untouched', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    const address = `${POL1.id}@${FACE.draft}`;
    await faces.openEntity('policy', address);

    await faces.clickEdit();
    await form.fillField('title', 'Access Control (revised draft)');
    await form.saveAndWaitForPatch('policies', address);

    await expect
      .poll(async () => (await facedApi.getPolicy(address))?.properties.title)
      .toBe('Access Control (revised draft)');
    const published = await facedApi.getPolicy(`${POL1.id}@${FACE.published}`);
    expect(published?.properties.title).toBe(POL1.publishedTitle);
  });

  test('the published face offers no Edit to a draft-only writer', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', `${POL1.id}@${FACE.published}`);

    await faces.expectHeading(POL1.publishedTitle);
    await faces.expectNoEdit();
  });

  test('+ New from the published list creates into the editorial world\'s draft face', async ({
    appPage,
    facedApi,
  }) => {
    const list = new ListPage(appPage);
    const form = new FormPage(appPage);
    await list.navigateToList('policies');

    await list.clickCreateButton();
    await expect(appPage).toHaveURL(new RegExp(`world=${WORLD.editorial}`));
    await form.fillField('title', 'Acceptable Use');
    const created = await form.submitAndExpectCreate('policies');

    const draft = await facedApi.getPolicy(`${created.id}@${FACE.draft}`);
    expect(draft?.properties.title).toBe('Acceptable Use');
    expect(await facedApi.getPolicy(`${created.id}@${FACE.published}`)).toBeNull();
  });

  test('Publish copies a draft-only policy into the published world', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    const list = new ListPage(appPage);
    await faces.openEntity('policy', `${POL2.id}@${FACE.draft}`);

    await faces.invokeCopy('Publish', `${POL2.id}@${FACE.published}`);
    await faces.expectHeading(POL2.title);

    await list.navigateToList('policies');
    await list.expectRowCount(2);
    await list.expectCellInRow(POL2.id, POL2.title);
  });

  // POL-2 is used because it has no edges. Deleting a face that has a
  // content-scoped edge is refused today; see faces-backlog.spec.ts.
  test('deleting the draft face keeps the published face', async ({ appPage, facedApi }) => {
    const faces = new FacesPage(appPage);
    await facedApi.invokeCopy('publish', POL2.id);
    await faces.openEntity('policy', `${POL2.id}@${FACE.draft}`);

    await faces.deleteShownFace('Draft');

    await expect.poll(() => facedApi.getPolicy(`${POL2.id}@${FACE.draft}`)).toBeNull();
    const published = await facedApi.getPolicy(`${POL2.id}@${FACE.published}`);
    expect(published?.properties.title).toBe(POL2.title);
  });

  test('a comment on the draft face stays on the draft face', async ({ appPage, facedApi }) => {
    const comments = new CommentsPage(appPage);
    const draft = `${POL1.id}@${FACE.draft}`;
    await comments.openEntity('policy', draft);

    await comments.openField('owner');
    await comments.postFieldComment('Who owns the draft?');

    await expect(comments.fieldCommentBodies()).toHaveText(['Who owns the draft?']);
    expect((await facedApi.listComments('policy', draft)).map((c) => c.body)).toEqual([
      'Who owns the draft?',
    ]);
    expect(await facedApi.listComments('policy', `${POL1.id}@${FACE.published}`)).toEqual([]);
  });
});
