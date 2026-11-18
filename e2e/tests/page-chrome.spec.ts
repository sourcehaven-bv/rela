import fs from 'node:fs';
import path from 'node:path';
import { test as base, expect } from './fixtures';
import { DashboardPage, EntityPage, PilesPage } from '../pages';

/**
 * Page chrome toggles (TKT-31822PN). An operator hides the dashboard's
 * validation card with `validation:` and the entity header's "Add to pile"
 * with `entity_header.add_to_pile`, or gates either on a permission.
 *
 * A permission gate is a UX filter, not access control. With no acl.yaml
 * there is no permission model, so a gated element shows.
 */
const CHROME_YAML = `
dashboards:
  portal:
    title: "Portal"
    validation: false
    cards:
      - title: "Features"
        query: "type:feature"
        display: count
entity_header:
  add_to_pile:
    permission: staff.manage
`;

// The e2e user may read and write everything but does not hold staff.manage.
const NO_STAFF_ACL = `roles:
  customer:
    create: ["*"]
    read: ["*"]
    update: ["*"]
assignments:
  e2e@example.com: customer
`;

// The same user, now holding staff.manage through its role.
const STAFF_ACL = `roles:
  staff:
    create: ["*"]
    read: ["*"]
    update: ["*"]
    permissions: [staff.manage]
assignments:
  e2e@example.com: staff
`;

const test = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.appendFileSync(path.join(testProject, 'data-entry.yaml'), CHROME_YAML);
    await use(testProject);
  },
});

const testNoStaff = test.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, 'acl.yaml'), NO_STAFF_ACL);
    await use(testProject);
  },
});

test.describe('Page chrome toggles', () => {
  test('a dashboard with validation: false has no validation card', async ({ appPage }) => {
    const dashboard = new DashboardPage(appPage);

    // The default dashboard does not set the key and keeps the card.
    await dashboard.navigate();
    await dashboard.expectValidationCard(true);

    await dashboard.navigateNamed('portal');
    await dashboard.expectCardTitles(['Features']);
    await dashboard.expectValidationCard(false);
  });

  test('a gated Add to pile shows without acl.yaml', async ({ appPage, api }) => {
    const created = await api.createEntity('features', { properties: { title: 'Chrome shown' } });
    const entity = new EntityPage(appPage);
    const piles = new PilesPage(appPage);

    await entity.navigateToEntity('feature', created.id);
    await entity.expectHeadingText('Chrome shown');
    await expect(piles.addToPileButton).toBeVisible();
  });
});

const testStaff = test.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, 'acl.yaml'), STAFF_ACL);
    await use(testProject);
  },
});

// The positive control for the test below: under acl.yaml, a holder of the
// permission sees the button, so its absence there comes from the gate.
testStaff('a gated Add to pile shows for a holder of the permission', async ({ appPage, api }) => {
  const created = await api.createEntity('features', { properties: { title: 'Chrome staff' } });
  const entity = new EntityPage(appPage);
  const piles = new PilesPage(appPage);

  await entity.navigateToEntity('feature', created.id);
  await entity.expectHeadingText('Chrome staff');
  await expect(piles.addToPileButton).toBeVisible();
});

testNoStaff('a gated Add to pile is absent for a principal without the permission', async ({
  appPage,
  api,
}) => {
  const created = await api.createEntity('features', { properties: { title: 'Chrome hidden' } });
  const entity = new EntityPage(appPage);
  const piles = new PilesPage(appPage);

  await entity.navigateToEntity('feature', created.id);
  await entity.expectHeadingText('Chrome hidden');
  // The Edit button proves the header's action row has rendered.
  expect(await entity.hasEditButton()).toBe(true);
  await expect(piles.addToPileButton).toHaveCount(0);
});
