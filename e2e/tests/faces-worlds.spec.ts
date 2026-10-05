import { facedTest as test, expect, FACE, WORLD } from './faced-fixtures';
import { FacesPage, FormPage, ListPage } from '../pages';

/**
 * Choosing a world and creating into a face (TKT-7IZHP0 §15). Runs as the
 * editor, whose create grant names `policy@draft` only.
 */
test.describe('Faces: worlds', () => {
  test('the world switcher writes ?world= and drops it for the default world', async ({
    appPage,
  }) => {
    const faces = new FacesPage(appPage);
    const list = new ListPage(appPage);
    await list.navigateToList('policies');
    await faces.expectSelectedWorld(WORLD.published);

    await faces.selectWorld(WORLD.editorial);
    await expect.poll(() => faces.worldParam()).toBe(WORLD.editorial);
    await faces.expectWorldBanner('Editorial view');

    await faces.selectWorld(WORLD.published);
    await expect.poll(() => faces.worldParam()).toBeNull();
  });

  test('a stale ?world=default is dropped and the default world is served', async ({
    appPage,
  }) => {
    const faces = new FacesPage(appPage);
    const list = new ListPage(appPage);
    await list.navigateTo('/list/policies?world=default');

    await expect.poll(() => faces.worldParam()).toBeNull();
    await faces.expectSelectedWorld(WORLD.published);
    await faces.expectWorldBanner('Published view');
  });

  test('a create from a world without create: asks for a face the editor may create', async ({
    appPage,
    facedApi,
  }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('policy');

    expect(await faces.createFaceOptions()).toEqual(['Draft']);
    await form.fillField('title', 'Remote Work');
    const created = await form.submitAndExpectCreate('policies');

    const draft = await facedApi.getPolicy(`${created.id}@${FACE.draft}`);
    expect(draft?.properties.title).toBe('Remote Work');
  });

  test('a create from a world with create: asks nothing', async ({ appPage }) => {
    const faces = new FacesPage(appPage);
    const form = new FormPage(appPage);
    await form.navigateTo(`/form/policy?world=${WORLD.editorial}`);
    await form.waitForSpinnerToDisappear();

    await faces.expectNoCreateFacePicker();
  });
});
