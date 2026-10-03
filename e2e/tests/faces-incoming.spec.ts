import { facedTest as test, expect, FACED_SEED, FACE } from './faced-fixtures';
import { FormPage } from '../pages';

/**
 * Incoming content-scoped edges per source face. POL-1@published implements
 * CTL-3 in the seed. The editor may write `policy@draft` only, so on CTL-3's
 * form the published edge is locked and only the draft face is offered.
 */
const POL1 = FACED_SEED.both;
const CTL3 = FACED_SEED.controls.spare;

test.describe('Faces: incoming edges per face', () => {
  test('a form adds and removes one face\'s edge and keeps the other', async ({
    appPage,
    facedApi,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToEditForm('control', CTL3.id);
    const picker = form.relationPickerByLabel('Implemented by');

    // The published edge shows, locked: the editor cannot write that face.
    const published = form.pickerFaceGroup(picker, 'Published');
    await expect(published).toContainText(POL1.id);
    await expect(form.pickerLocks(published)).toHaveCount(1);
    await expect(form.pickerRemoveButtons(published)).toHaveCount(0);

    // Only the draft face is offered.
    await form.openRelationPicker(picker);
    const options = form.pickerOptions(picker).filter({ hasText: POL1.id });
    await expect(options).toHaveCount(1);
    await expect(options).toContainText('Draft');
    await options.click();
    await form.saveAndWaitForPatch('controls', CTL3.id);

    await expect
      .poll(async () =>
        (await facedApi.listIncoming('controls', CTL3.id, 'implements'))
          .map((e) => `${e.id}@${e.face}`)
          .sort(),
      )
      .toEqual([`${POL1.id}@${FACE.draft}`, `${POL1.id}@${FACE.published}`]);

    // Reloaded, the two edges are grouped per face; removing the draft one
    // leaves the published one.
    await form.navigateToEditForm('control', CTL3.id);
    await expect(form.pickerFaceGroup(picker, 'Draft')).toContainText(POL1.id);
    expect((await form.pickerFaceLabels(picker)).sort()).toEqual(['Draft', 'Published']);
    await form.pickerRemoveButtons(form.pickerFaceGroup(picker, 'Draft')).click();
    await form.saveAndWaitForPatch('controls', CTL3.id);

    await expect
      .poll(async () =>
        (await facedApi.listIncoming('controls', CTL3.id, 'implements')).map(
          (e) => `${e.id}@${e.face}`,
        ),
      )
      .toEqual([`${POL1.id}@${FACE.published}`]);
  });
});
