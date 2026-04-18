import { test, expect } from './fixtures';
import { FormPage, ListPage, EntityPage, ApiClient } from '../pages';

test.describe('Entity CRUD Operations', () => {
  test.describe('Create Entity', () => {
    test('can create a new feature with all fields', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      // Navigate to features list
      await listPage.navigateToList('features');
      await listPage.expectHeading('Features');

      // Click create button
      await listPage.clickCreateButton();
      await formPage.expectFormTitle('Feature');

      // Fill in the form
      await formPage.fillField('title', 'New E2E Feature');
      await formPage.selectField('status', 'approved');
      await formPage.selectField('priority', 'high');

      // Submit the form
      await formPage.submit();

      // Should redirect to entity view or list
      await expect(appPage).not.toHaveURL(/\/form\/feature$/);

      // Verify the feature appears in the list
      await listPage.navigateToList('features');
      await listPage.expectRowContains('New E2E Feature');
    });

    test('can create a bug with severity', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('bugs');
      await listPage.clickCreateButton();

      await formPage.fillField('title', 'Critical Security Bug');
      await formPage.selectField('severity', 'critical');
      await formPage.selectField('status', 'draft');

      await formPage.submit();

      await listPage.navigateToList('bugs');
      await listPage.expectRowContains('Critical Security Bug');
    });

    test('validates required fields', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickCreateButton();

      // Try to submit without title (required field)
      await formPage.submit();

      // Should show validation error or stay on form
      await expect(appPage).toHaveURL(/\/form\/feature/);
    });
  });

  test.describe('Read Entity', () => {
    test('can view entity details', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const entityPage = new EntityPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickRowById('FEAT-001');

      await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-001|\/form\/feature\/FEAT-001/);
      await entityPage.expectHeadingText('User Authentication');
    });

    test('existing entities are displayed in list', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      // Should show pre-created entities
      await listPage.expectRowContains('FEAT-001');
      await listPage.expectRowContains('User Authentication');
      await listPage.expectRowContains('FEAT-002');
      await listPage.expectRowContains('Dashboard Analytics');
    });

    test('can navigate via relation links', async ({ appPage, serverUrl }) => {
      const api = new ApiClient(appPage, serverUrl);
      const entityPage = new EntityPage(appPage);

      const feature = await api.createEntity('features', {
        title: 'RelationTestFeature',
        status: 'draft',
        priority: 'high',
      });
      const task = await api.createEntity('tasks', {
        title: 'Relation Navigation Test Task',
        status: 'draft',
        assignee: 'e2e-test',
      });
      await api.createRelation('tasks', task.id, 'implements', feature.id);

      await entityPage.navigateToEntity('task', task.id);
      await entityPage.expectHeadingText('Relation Navigation Test Task');
      await entityPage.clickRelationLink(feature.id);

      await expect(appPage).toHaveURL(new RegExp(`/entity/feature/${feature.id}`));
      await entityPage.expectTypeBadge('feature');
      await entityPage.expectHeadingText('RelationTestFeature');
    });
  });

  test.describe('Update Entity', () => {
    test('can edit an existing feature', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);
      const entityPage = new EntityPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickRowById('FEAT-002');
      await entityPage.clickEdit();

      await formPage.fillField('title', 'Updated Dashboard Analytics');
      await formPage.selectField('status', 'approved');
      await formPage.submit();

      await listPage.navigateToList('features');
      await listPage.expectRowContains('Updated Dashboard Analytics');
    });

    test('can change entity status', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);
      const entityPage = new EntityPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickRowById('FEAT-001');
      await entityPage.clickEdit();

      await formPage.selectField('status', 'done');
      await formPage.submit();

      await listPage.navigateToList('features');
      await listPage.expectCellInRow('FEAT-001', 'done');
    });
  });

  test.describe('Delete Entity', () => {
    test('can delete an entity from list', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickCreateButton();

      await formPage.fillField('title', 'Feature to Delete');
      await formPage.submit();

      await listPage.navigateToList('features');
      await listPage.expectRowContains('Feature to Delete');

      await listPage.deleteRowByTitle('Feature to Delete');

      await listPage.expectRowNotVisible('Feature to Delete');
    });

    test('delete confirmation can be cancelled', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const initialCount = await listPage.getRowCount();

      // Click delete on the first row then cancel the modal
      await listPage.openDeleteModalForFirstRow();
      await listPage.cancelDeleteModal();

      await expect(await listPage.getRowCount()).toBe(initialCount);
    });
  });
});
