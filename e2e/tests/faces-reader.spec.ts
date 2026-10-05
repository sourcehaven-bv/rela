import { facedTest as test, expect, FACED_SEED, FACED_USERS, FACE, WORLD } from './faced-fixtures';
import { FacesPage, ListPage } from '../pages';

/**
 * A `type@face` read grant (TKT-WCMW47). The reader holds `policy@published`,
 * `control` and `world:published`, so a draft face must be as absent as a
 * missing entity: not listed, not reachable by address, not reachable through
 * another world.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;

test.use({ facedUser: FACED_USERS.reader });

test.describe('Faces: published-only reader', () => {
  test('the list shows the published face only', async ({ appPage }) => {
    const list = new ListPage(appPage);
    await list.navigateToList('policies');

    await list.expectRowCount(1);
    await list.expectCellInRow(POL1.id, POL1.publishedTitle);
    await list.expectRowNotVisible(POL1.draftTitle);
    await list.expectRowNotVisible(POL2.title);
  });

  test('the published face renders without Edit or Publish', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', `${POL1.id}@${FACE.published}`);

    await faces.expectHeading(POL1.publishedTitle);
    await faces.expectNoEdit();
    await faces.expectNoCopy('Publish');
  });

  test('a draft face is not found by address', async ({ appPage }) => {
    const faces = new FacesPage(appPage);

    await faces.openEntity('policy', `${POL1.id}@${FACE.draft}`);
    await faces.expectNotFound();

    await faces.openEntity('policy', POL2.id);
    await faces.expectNotFound();
  });

  test('the API answers 404 for a hidden face, a draft-only entity and a missing id alike', async ({
    facedApi,
  }) => {
    // POL-2@draft is the address that tests the grant: bare POL-2 is also
    // excluded by the published world's `otherwise: exclude`.
    expect(await facedApi.getPolicy(`${POL1.id}@${FACE.draft}`)).toBeNull();
    expect(await facedApi.getPolicy(`${POL2.id}@${FACE.draft}`)).toBeNull();
    expect(await facedApi.getPolicy(POL2.id)).toBeNull();
    expect(await facedApi.getPolicy('POL-999')).toBeNull();
    expect(await facedApi.getPolicy(`${POL1.id}@${FACE.published}`)).not.toBeNull();
  });

  test('asking for the editorial world does not reveal drafts', async ({ appPage }) => {
    const list = new ListPage(appPage);
    await list.navigateToList('policies', `world=${WORLD.editorial}`);

    // The page rendered, and it rendered no rows.
    await list.expectEmpty();
  });
});
