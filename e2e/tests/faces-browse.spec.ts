import { facedTest as test, expect, FACED_SEED, WORLD } from './faced-fixtures';
import { FacesPage, ListPage } from '../pages';

/**
 * Browsing a faced type in declared worlds (TKT-WCMW47). Runs as the editor,
 * who reads every face and both worlds.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;
const CTL = FACED_SEED.controls;

test.describe('Faces: browse in a world', () => {
  test('the default world lists only published faces under their published titles', async ({
    appPage,
  }) => {
    const list = new ListPage(appPage);
    const faces = new FacesPage(appPage);
    await list.navigateToList('policies');

    await faces.expectWorldBanner('Published view');
    await list.expectRowCount(1);
    await list.expectCellInRow(POL1.id, POL1.publishedTitle);
    // The published title is a substring of the draft title, so the cell
    // check alone would also pass on the draft.
    await list.expectRowNotVisible(POL1.draftTitle);
    await list.expectRowNotVisible(POL2.title);
  });

  test('?world=editorial lists every policy under its draft title', async ({ appPage }) => {
    const list = new ListPage(appPage);
    const faces = new FacesPage(appPage);
    await list.navigateToList('policies', `world=${WORLD.editorial}`);

    await faces.expectWorldBanner('Editorial view');
    await list.expectRowCount(2);
    await list.expectCellInRow(POL1.id, POL1.draftTitle);
    await list.expectCellInRow(POL2.id, POL2.title);
  });

  test('row links carry the world they were listed in', async ({ appPage }) => {
    const list = new ListPage(appPage);
    await list.navigateToList('policies', `world=${WORLD.editorial}`);

    const href = await list.rowLinkHref(0);
    expect(href).toContain(`world=${WORLD.editorial}`);
  });

  test('an ID@face address shows that face whatever the world', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', `${POL1.id}@draft`);

    await faces.expectHeading(POL1.draftTitle);
    await faces.expectBodyContains(POL1.draftBody);
  });

  test('the face switcher moves from the draft to the published face', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', `${POL1.id}@draft`);

    await faces.switchToFace('Published');
    await faces.expectHeading(POL1.publishedTitle);
    await faces.expectBodyContains(POL1.publishedBody);
    await faces.expectBodyNotContains(POL1.draftBody);
  });

  test('a draft-only policy in the published world shows the absent banner', async ({
    appPage,
  }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('policy', POL2.id, WORLD.published);

    await faces.expectAbsentBanner();
    await expect(faces.copyButton('Publish')).toBeVisible();
  });

  test('an identity-scoped edge shows on the faceless target', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    await faces.openEntity('control', CTL.visitors.id, WORLD.published);

    await expect(faces.relationCard(POL1.id)).toBeVisible();
  });
});
