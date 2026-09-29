import { facedPgTest as test, expect, FACE, POSTGRES_E2E_ENABLED } from './faced-fixtures';
import { HistoryPage } from '../pages';

/**
 * Version history and restore on one face (TKT-WCMW47). History is a
 * database-backend capability, so this runs on postgres and skips without
 * RELA_E2E_DATABASE_URL. The postgres store starts empty, so the test seeds
 * through the API.
 */
test.describe('History on a face (BUG-4SYAA6)', () => {
  test.skip(!POSTGRES_E2E_ENABLED, 'requires a postgres backend (set RELA_E2E_DATABASE_URL)');

  test('restoring an old version of the draft face restores that face only', async ({
    appPage,
    api,
    facedApi,
  }) => {
    // Three sweeps and a timeline poll can outlast the default timeout.
    test.slow();
    const created = await facedApi.createPolicy(FACE.draft, { title: 'Travel' });
    const draft = `${created.id}@${FACE.draft}`;
    const published = `${created.id}@${FACE.published}`;
    await api.waitForEntityVersions('policy', draft, 1);
    await facedApi.updatePolicy(draft, { title: 'Travel v2' });
    await api.waitForEntityVersions('policy', draft, 2);
    // A published face with the later title, so a restore that crosses faces
    // is visible.
    await facedApi.invokeCopy('publish', created.id);
    await api.waitForEntityVersions('policy', published, 1);

    const history = new HistoryPage(appPage);
    await history.open('policy', draft);
    await history.waitForTimelineAtLeast(2);
    await history.restoreVersion(1);

    await expect
      .poll(async () => (await facedApi.getPolicy(draft))?.properties.title)
      .toBe('Travel');
    expect((await facedApi.getPolicy(published))?.properties.title).toBe('Travel v2');
  });
});
