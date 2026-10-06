import { facedTest as test, expect, FACED_SEED, WORLD } from './faced-fixtures';
import { FacesPage, FormPage } from '../pages';

/**
 * The edit form reads an entity in the world the detail page showed it in.
 * A faceless control links to a draft-only policy. The default world serves
 * published faces only, so it hides that link; the editorial world shows it.
 * An edit form opened from the editorial world must show the link, keep it
 * when another is added, and let the user remove it.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;
const CTL = FACED_SEED.controls.spare;

test.describe('Faces: edit form relations in a world', () => {
  test.beforeEach(async ({ facedApi }) => {
    await facedApi.link('controls', CTL.id, 'mitigates', POL2.id);
  });

  test('Edit from a world shows the links that world serves and saves against them', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);

    await faces.openEntity('control', CTL.id, WORLD.editorial);
    await expect(faces.relationCard(POL2.id)).toBeVisible();
    await faces.clickEdit();

    expect(faces.worldParam()).toBe(WORLD.editorial);
    const picker = form.relationPickerByLabel('Mitigates');
    await expect(form.pickerTileByText(picker, POL2.title)).toBeVisible();

    await form.pickInRelationPicker(picker, 'Access', POL1.draftTitle);
    await form.saveAndWaitForPatch('controls', CTL.id);
    expect(await facedApi.outgoingIds('controls', CTL.id, 'mitigates', WORLD.editorial)).toEqual(
      [POL1.id, POL2.id].sort(),
    );

    // Only a form that loaded the link can remove it.
    await form.removePickerTile(picker, POL2.title);
    await form.saveAndWaitForPatch('controls', CTL.id);
    expect(await facedApi.outgoingIds('controls', CTL.id, 'mitigates', WORLD.editorial)).toEqual([
      POL1.id,
    ]);
  });

  test('switching world in an open form reloads its links in that world', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await faces.openEntity('control', CTL.id, WORLD.editorial);
    await faces.clickEdit();
    const picker = form.relationPickerByLabel('Mitigates');
    await expect(form.pickerTileByText(picker, POL2.title)).toBeVisible();

    await faces.selectWorld(WORLD.published);

    await expect(form.pickerTileByText(picker, POL2.title)).toHaveCount(0);
  });
});
