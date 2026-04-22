import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

test.describe('Entity Detail View', () => {
  let testFeatureId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const feature = await api.createEntity('features', {
      properties: {
        title: 'Detail View Test Feature',
        description: 'Testing the entity detail view functionality',
        status: 'draft',
        priority: 'high',
      },
    });
    testFeatureId = feature.id;
  });

  test.afterEach(async ({ api }) => {
    if (testFeatureId) {
      await api.deleteEntity('features', testFeatureId).catch(() => {});
      testFeatureId = null;
    }
  });

  test('displays entity detail page', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
  });

  test('shows entity title', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
    expect(await entity.containsText('Detail View Test Feature')).toBeTruthy();
  });

  test('shows entity properties', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
    await entity.expectHeadingText('Detail View Test Feature');
    expect(await entity.hasPropertyValue('draft')).toBeTruthy();
    expect(await entity.hasPropertyValue('high')).toBeTruthy();
  });

  test('shows entity description', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
    expect(await entity.containsText('Testing the entity detail view functionality')).toBeTruthy();
  });

  test('has edit button', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
    expect(await entity.hasEditButton()).toBeTruthy();
  });

  test('edit button navigates to edit form', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', testFeatureId!);
    await entity.clickEdit();
    await expect(appPage).toHaveURL(/\/form\/feature\//);
  });
});

test.describe('Bug Detail View', () => {
  let testBugId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const bug = await api.createEntity('bugs', {
      properties: {
        title: 'Bug Detail Test',
        description: 'Testing bug detail view',
        severity: 'high',
        status: 'draft',
        priority: 'high',
      },
    });
    testBugId = bug.id;
  });

  test.afterEach(async ({ api }) => {
    if (testBugId) {
      await api.deleteEntity('bugs', testBugId).catch(() => {});
      testBugId = null;
    }
  });

  test('displays bug detail page', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', testBugId!);
    expect(await entity.containsText('Bug Detail Test')).toBeTruthy();
  });

  test('shows bug description', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('bug', testBugId!);
    expect(await entity.containsText('Testing bug detail view')).toBeTruthy();
  });
});

test.describe('Entity Detail with Relations', () => {
  let featureId: string | null = null;
  let blockingFeatureId: string | null = null;

  test.beforeEach(async ({ api }) => {
    const blocker = await api.createEntity('features', {
      properties: { title: 'Blocking Feature', status: 'draft', priority: 'high' },
    });
    blockingFeatureId = blocker.id;

    const blocked = await api.createEntity('features', {
      properties: { title: 'Blocked Feature', status: 'draft', priority: 'medium' },
    });
    featureId = blocked.id;

    await api.createRelation('features', blockingFeatureId, 'blocks', featureId);
  });

  test.afterEach(async ({ api }) => {
    if (featureId) await api.deleteEntity('features', featureId).catch(() => {});
    if (blockingFeatureId) await api.deleteEntity('features', blockingFeatureId).catch(() => {});
    featureId = null;
    blockingFeatureId = null;
  });

  test('shows blocking relations on the blocked feature', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', featureId!);
    expect(await entity.hasBlockingRelationsSection()).toBeTruthy();
  });

  test('shows blocks relations on the blocking feature', async ({ appPage }) => {
    const entity = new EntityPage(appPage);
    await entity.navigateToEntity('feature', blockingFeatureId!);
    expect(await entity.hasBlockingRelationsSection()).toBeTruthy();
  });
});
