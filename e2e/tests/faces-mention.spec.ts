import {
  facedTest as test,
  facedPgTest,
  expect,
  FACE,
  FACED_SEED,
  POSTGRES_E2E_ENABLED,
} from './faced-fixtures';
import { FormPage } from '../pages';

/**
 * The `@` menu finds a faced entity from a faceless entity's body.
 *
 * `policy` has faces and no default face; the default world serves its
 * published face. The policies are seeded as files, so the search index learns
 * them from the startup backfill rather than from a write.
 */
const POL1 = FACED_SEED.both;
const POL2 = FACED_SEED.draftOnly;

test.describe('Faces: @ mention of a faced type', () => {
  test('a new control finds a policy through the type scope and a search', async ({
    appPage,
  }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('control');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    // A partial type name offers the type.
    await form.typeIntoEditor('see @poli');
    await form.waitForMentionMenu();
    const policyType = form.mentionMenuTypeOptions.filter({ hasText: 'policy' });
    await expect(policyType).toHaveCount(1);

    // Choosing it lists the policies the world serves.
    await policyType.first().click();
    await expect(form.mentionMenuScopeChip).toHaveText('policy');
    await expect(form.mentionMenuEntityOptions.filter({ hasText: POL1.id })).toContainText(
      POL1.publishedTitle,
      { timeout: 5_000 },
    );

    // A search under the scope finds it by title. A whole word: the file
    // backend's index matches words, not prefixes.
    await appPage.keyboard.type('Access');
    await expect(form.mentionMenuEntityOptions.first()).toContainText(POL1.id, {
      timeout: 5_000,
    });
    await expect(form.mentionMenuEntityOptions.first()).toContainText(POL1.publishedTitle);
  });

  test('an unscoped search finds a policy by title', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('control');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    await form.typeIntoEditor('see @Access');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuEntityOptions.first()).toContainText(POL1.id, {
      timeout: 5_000,
    });
  });

  test('a search does not find a policy the world excludes', async ({ appPage }) => {
    const form = new FormPage(appPage);
    await form.navigateToCreateForm('control');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    await form.typeIntoEditor('see @policy:Retention');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuNote).toHaveText('No matches', { timeout: 5_000 });
    await expect(form.mentionMenuEntityOptions.filter({ hasText: POL2.id })).toHaveCount(0);
  });
});

// The same flow on postgres, which searches in SQL and matches a partial word.
// The store starts empty, so the policy is seeded and published through the API.
facedPgTest.describe('Faces: @ mention of a faced type on postgres', () => {
  facedPgTest.skip(!POSTGRES_E2E_ENABLED, 'requires a postgres backend (set RELA_E2E_DATABASE_URL)');

  facedPgTest('a scoped partial word finds the published policy', async ({
    appPage,
    facedApi,
  }) => {
    const created = await facedApi.createPolicy(FACE.draft, { title: 'Offboarding' });
    await facedApi.invokeCopy('publish', created.id);

    const form = new FormPage(appPage);
    await form.navigateToCreateForm('control');
    await form.expectMarkdownEditorReady();
    await form.clearEditorBuffer();

    await form.typeIntoEditor('see @policy:Offb');
    await form.waitForMentionMenu();
    await expect(form.mentionMenuEntityOptions.first()).toContainText(created.id, {
      timeout: 5_000,
    });
    await expect(form.mentionMenuEntityOptions.first()).toContainText('Offboarding');
  });
});
