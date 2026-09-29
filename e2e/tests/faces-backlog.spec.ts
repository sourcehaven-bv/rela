import * as fs from 'fs';
import { facedTest as test, expect, FACED_SEED, FACED_USERS, FACE, WORLD } from './faced-fixtures';
import { CommentsPage, EntityPage, FacesPage, FormPage } from '../pages';

/**
 * Face and world flows that are broken today (TKT-WCMW47). Every test is
 * `test.fixme` and names the defect that blocks it. The bodies assert the
 * FIXED behaviour, so the fix for a bug turns its test on by deleting
 * `.fixme` and nothing else. A test that still fails after that is evidence
 * the fix is incomplete, not that the test is wrong.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;
const CTL = FACED_SEED.controls;
const POL1_DRAFT = `${POL1.id}@${FACE.draft}`;
const POL1_PUBLISHED = `${POL1.id}@${FACE.published}`;

test.describe('Faces backlog: attachments and export (BUG-CTUW2N)', () => {
  test.fixme('a file attached to the draft face stays on the draft face', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await form.navigateToEditForm('policy', POL1_DRAFT);

    await form.attachFileAndWaitForUpload('evidence', 'draft-evidence.txt', 'draft only');

    await faces.openEntity('policy', POL1_DRAFT);
    await faces.expectBodyContains('draft-evidence.txt');
    await faces.openEntity('policy', POL1_PUBLISHED);
    await faces.expectBodyNotContains('draft-evidence.txt');
  });

  test.fixme('Publish carries the draft face\'s attachment to the published face', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await form.navigateToEditForm('policy', `${POL2.id}@${FACE.draft}`);
    await form.attachFileAndWaitForUpload('evidence', 'schedule.txt', 'retention table');

    await facedApi.invokeCopy('publish', POL2.id);

    await faces.openEntity('policy', `${POL2.id}@${FACE.published}`);
    await faces.expectBodyContains('schedule.txt');
  });

  test.fixme('export of the draft face renders the draft', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1_DRAFT);

    const download = await faces.exportAs('markdown');
    const text = fs.readFileSync(await download.path(), 'utf8');

    expect(text).toContain(POL1.draftTitle);
    expect(text).not.toContain(POL1.publishedBody);
  });
});

test.describe('Faces backlog: anchored documents (BUG-8J3LSB)', () => {
  test.fixme('the document on the draft face renders the draft', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1_DRAFT);

    await faces.expectDocumentContains(POL1.draftBody);
  });

  test.describe('as the published-only reader', () => {
    test.use({ facedUser: FACED_USERS.reader });

    test.fixme('the document renders the published face and never the draft', async ({
      appPage,
    }) => {
      const faces = new FacesPage(appPage);
      await faces.openEntity('policy', POL1.id, WORLD.published);

      await faces.expectDocumentContains(POL1.publishedBody);
      await expect(faces.documentsPanel()).not.toContainText(POL1.draftBody);
    });
  });
});

test.describe('Faces backlog: comments panel in a world (BUG-FYEEVX)', () => {
  test.fixme('the bare address in a world comments on the face that world serves', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    const comments = new CommentsPage(appPage);
    await faces.openEntity('policy', POL1.id, WORLD.editorial);
    await expect(faces.commentsPanelSummary()).toHaveText('0 total');

    await comments.openField('owner');
    await comments.postFieldComment('Draft owner?');

    const onDraft = await facedApi.listComments('policy', POL1_DRAFT);
    expect(onDraft.map((c) => c.body)).toEqual(['Draft owner?']);
    expect(await facedApi.listComments('policy', POL1_PUBLISHED)).toEqual([]);
  });
});

test.describe('Faces backlog: duplicate and relations to a faced target (BUG-FYEEVX, BUG-BZQQDP)', () => {
  test.fixme('duplicating the draft face creates a new draft', async ({ appPage, facedApi }) => {
    const entity = new EntityPage(appPage);
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1_DRAFT);

    await entity.openDuplicate();
    await entity.continueDuplicate();
    await entity.submitDuplicateForm();

    const newId = (await faces.landedAddressOtherThan('policy', POL1.id)).split('@')[0];
    expect(await facedApi.getPolicy(`${newId}@${FACE.draft}`)).not.toBeNull();
    expect(await facedApi.getPolicy(`${newId}@${FACE.published}`)).toBeNull();
  });

  test.fixme('a control can relate to a draft-only policy', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await form.navigateToEditForm('control', CTL.visitors.id);

    const picker = form.relationPickerByLabel('Mitigates');
    await form.pickInRelationPicker(picker, 'Retention', POL2.title);
    await form.saveAndWaitForPatch('controls', CTL.visitors.id);

    await faces.openEntity('control', CTL.visitors.id, WORLD.editorial);
    await expect(faces.relationCard(POL2.id)).toBeVisible();
  });
});

test.describe('Faces backlog: content-scoped edges per face (untracked, found by TKT-WCMW47)', () => {
  // `_views` lists both faces' `implements` edges on either face, and the
  // faceless target shows an edge from a face the world does not serve.
  test.fixme('the draft face shows only its own implements edge', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1_DRAFT);

    await expect(faces.relationCard(CTL.badge.id)).toBeVisible();
    await expect(faces.relationCard(CTL.spare.id)).toHaveCount(0);
  });

  test.fixme('a control does not show an edge from a face the world hides', async ({
    appPage,
  }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('control', CTL.badge.id, WORLD.published);

    await faces.waitForSection('implemented-by');
    await expect(faces.relationCard(POL1.id)).toHaveCount(0);
  });

  // For the reader this is a disclosure, not a world bug: CTL-1 is linked
  // only from POL-1@draft, which the reader may not read.
  test.describe('as the published-only reader', () => {
    test.use({ facedUser: FACED_USERS.reader });

    test.fixme('the published face does not show the draft face\'s edge', async ({ appPage }) => {
      const faces = new FacesPage(appPage);
      await faces.openEntity('policy', POL1_PUBLISHED);

      await expect(faces.relationCard(CTL.spare.id)).toBeVisible();
      await expect(faces.relationCard(CTL.badge.id)).toHaveCount(0);
    });

    test.fixme('a control does not show an edge from an unreadable draft', async ({ appPage }) => {
      const faces = new FacesPage(appPage);
      await faces.openEntity('control', CTL.badge.id, WORLD.published);

      await faces.waitForSection('implemented-by');
      await expect(faces.relationCard(POL1.id)).toHaveCount(0);
    });
  });
});

test.describe('Faces backlog: export as the published-only reader (BUG-CTUW2N)', () => {
  test.use({ facedUser: FACED_USERS.reader });

  test.fixme('export renders the published face and never the draft', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1.id, WORLD.published);

    const download = await faces.exportAs('markdown');
    const text = fs.readFileSync(await download.path(), 'utf8');

    expect(text).toContain(POL1.publishedBody);
    expect(text).not.toContain(POL1.draftBody);
  });
});

test.describe('Faces backlog: face delete with a content-scoped edge (untracked, found by TKT-WCMW47)', () => {
  // DELETE /policies/POL-1@draft answers 403 "no role grants delete on
  // relations from type \"\"": the cascade check resolves the edge source by
  // its bare id, which a faced type does not store.
  test.fixme('deleting the draft face removes its edges and keeps the published face', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL1_DRAFT);

    await faces.deleteShownFace('Draft');

    await expect.poll(() => facedApi.getPolicy(POL1_DRAFT)).toBeNull();
    // The published face keeps its own content edge and the shared identity
    // edge; only the draft's content edge goes.
    await faces.openEntity('policy', POL1_PUBLISHED);
    await expect(faces.relationCard(CTL.spare.id)).toBeVisible();
    await expect(faces.relationCard(CTL.visitors.id)).toBeVisible();
    await expect(faces.relationCard(CTL.badge.id)).toHaveCount(0);
  });
});

test.describe('Faces backlog: family delete under a draft-only grant (BUG-1YN750)', () => {
  // `actions/retire.lua` deletes by bare id, which is a family delete. The
  // editor's delete grant names `policy@draft` only, so it must be refused.
  // POL-2 is published first so it has two faces and no edges: POL-1's
  // content edges would trip the cascade defect above and refuse the delete
  // for the wrong reason.
  test.fixme('a draft-only deleter cannot remove the published face', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    await facedApi.invokeCopy('publish', POL2.id);
    await faces.openEntity('policy', `${POL2.id}@${FACE.draft}`);

    // Fixed behaviour: the action runs and the delete is refused. A script
    // that silently succeeds, or a success toast, is the bug.
    await faces.runAction('Retire');
    await faces.expectErrorToast();

    expect(await facedApi.getPolicy(`${POL2.id}@${FACE.published}`)).not.toBeNull();
    expect(await facedApi.getPolicy(`${POL2.id}@${FACE.draft}`)).not.toBeNull();
  });
});
