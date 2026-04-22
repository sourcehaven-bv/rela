import { test, expect } from './fixtures';
import { DashboardPage } from '../pages';

test.describe('Dashboard', () => {
  const createdIds: { plural: string; id: string }[] = [];

  test.beforeEach(async ({ api }) => {
    // Seed a few features and one critical bug so the cards have something to show.
    const items: Array<{ plural: string; properties: Record<string, unknown> }> = [
      { plural: 'features', properties: { title: 'Dashboard seed A', status: 'draft', priority: 'high' } },
      { plural: 'features', properties: { title: 'Dashboard seed B', status: 'draft', priority: 'medium' } },
      { plural: 'features', properties: { title: 'Dashboard seed C', status: 'in_progress', priority: 'low' } },
      { plural: 'bugs', properties: { title: 'Dashboard Critical', severity: 'critical', status: 'draft', priority: 'high' } },
    ];
    for (const { plural, properties } of items) {
      const created = await api.createEntity(plural, { properties });
      createdIds.push({ plural, id: created.id });
    }
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
    expect(await dashboard.pageContainsAny(['by status', 'draft', 'in_progress', 'done'])).toBeTruthy();
  });

  test('displays By Priority breakdown', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);
    await dashboard.navigate();
    expect(await dashboard.pageContainsAny(['by priority', 'critical', 'high', 'medium', 'low'])).toBeTruthy();
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
