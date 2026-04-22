import { test, expect, STATUS, SEVERITY, PRIORITY } from './fixtures';
import { DashboardPage } from '../pages';

test.describe('Dashboard', () => {
  const createdIds: { plural: string; id: string }[] = [];

  test.beforeEach(async ({ api }) => {
    // Seed a few features and one critical bug so the cards have something to show.
    const items: Array<{ plural: string; properties: Record<string, unknown> }> = [
      { plural: 'features', properties: { title: 'Dashboard seed A', status: STATUS.feature.draft, priority: PRIORITY.high } },
      { plural: 'features', properties: { title: 'Dashboard seed B', status: STATUS.feature.draft, priority: PRIORITY.medium } },
      { plural: 'features', properties: { title: 'Dashboard seed C', status: STATUS.feature.in_progress, priority: PRIORITY.low } },
      { plural: 'bugs', properties: { title: 'Dashboard Critical', severity: SEVERITY.critical, status: STATUS.feature.draft, priority: PRIORITY.high } },
    ];
    for (const { plural, properties } of items) {
      const created = await api.createEntity(plural, { properties });
      createdIds.push({ plural, id: created.id });
    }
    // Dashboard cards read via /_search; wait for the last seed to show up
    // in the index before any test navigates. Prevents the classic
    // creation+index race (RR-WB5VS).
    await api.waitForIndexed(createdIds[createdIds.length - 1].id);
  });

  test.afterEach(async ({ api }) => {
    while (createdIds.length) {
      const item = createdIds.pop()!;
      await api.deleteEntity(item.plural, item.id).catch(() => {});
    }
  });

  test('displays dashboard with cards', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(await dashboard.getCardCount()).toBeGreaterThanOrEqual(1);
  });

  test('displays Open Features count card', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(await dashboard.getStatValue('Open Features')).toMatch(/\d+/);
  });

  test('displays In Progress count card', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(await dashboard.getStatValue('In Progress')).toMatch(/\d+/);
  });

  test('displays By Status breakdown', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(
      await dashboard.pageContainsAny([
        'by status',
        STATUS.feature.draft,
        STATUS.feature.in_progress,
        STATUS.feature.done,
      ]),
    ).toBeTruthy();
  });

  test('displays By Priority breakdown', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(
      await dashboard.pageContainsAny([
        'by priority',
        SEVERITY.critical,
        PRIORITY.high,
        PRIORITY.medium,
        PRIORITY.low,
      ]),
    ).toBeTruthy();
  });

  test('Critical Issues table card shows the critical bug', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    await dashboard.expectCardTextContains('Critical Issues', 'Dashboard Critical');
  });

  test('clicking an entity link in a card navigates to detail', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    if (await dashboard.hasEntityLinkInCard('Critical Issues')) {
      await dashboard.clickFirstEntityLinkInCard('Critical Issues');
      await expect(appPage).toHaveURL(/\/entity\/bug\/BUG-\d+/);
    }
  });

  test('dashboard shows app title', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(await dashboard.pageContainsAny(/Dashboard|Feature\/bug overview/i)).toBeTruthy();
  });
});
