import { test, expect } from './fixtures';
import { FormPage } from '../pages/form.page';
import { ListPage } from '../pages/list.page';

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

      await listPage.navigateToList('features');

      // Click on existing feature
      await listPage.clickRowById('FEAT-001');

      // Should navigate to entity view
      await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-001|\/form\/feature\/FEAT-001/);

      // Should show entity details
      await expect(appPage.getByText('User Authentication')).toBeVisible();
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
      // Create a feature
      const featureResp = await appPage.request.post(`${serverUrl}/api/v1/features`, {
        data: {
          properties: {
            title: 'RelationTestFeature',
            status: 'draft',
            priority: 'high',
          },
        },
      });
      expect(featureResp.ok()).toBeTruthy();
      const feature = await featureResp.json();

      // Create a task that implements the feature
      const taskResp = await appPage.request.post(`${serverUrl}/api/v1/tasks`, {
        data: {
          properties: {
            title: 'Relation Navigation Test Task',
            status: 'draft',
            assignee: 'e2e-test',
          },
        },
      });
      expect(taskResp.ok()).toBeTruthy();
      const task = await taskResp.json();

      // Create relation from task to feature (implements)
      // V1 API: POST /api/v1/{plural}/{id}/relations/{relType} with { id: targetId }
      const relResp = await appPage.request.post(
        `${serverUrl}/api/v1/tasks/${task.id}/relations/implements`,
        { data: { id: feature.id } }
      );
      expect(relResp.ok()).toBeTruthy();

      // Navigate to the task entity page
      await appPage.goto(`${serverUrl}/v2/entity/task/${task.id}`);
      await expect(appPage.locator('h1').filter({ hasText: 'Relation Navigation Test Task' })).toBeVisible({ timeout: 10000 });

      // Find and click the relation link to the feature
      const relationLink = appPage.locator('button.relation-link').filter({ hasText: feature.id });
      await expect(relationLink).toBeVisible({ timeout: 5000 });
      await relationLink.click();

      // Should navigate to the feature entity page
      await expect(appPage).toHaveURL(new RegExp(`/entity/feature/${feature.id}`), { timeout: 10000 });
      await expect(appPage.locator('.entity-type-badge, [class*="badge"]').filter({ hasText: /feature/i })).toBeVisible({ timeout: 5000 });
      await expect(appPage.locator('h1').filter({ hasText: 'RelationTestFeature' })).toBeVisible();
    });
  });

  test.describe('Update Entity', () => {
    test('can edit an existing feature', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('features');

      // Click on existing feature to edit
      await listPage.clickRowById('FEAT-002');

      // Navigate to edit form if not already there
      if (!(await appPage.url()).includes('/form/')) {
        await appPage.click('a:has-text("Edit"), button:has-text("Edit")');
      }

      // Update the title
      await formPage.fillField('title', 'Updated Dashboard Analytics');
      await formPage.selectField('status', 'approved');

      await formPage.submit();

      // Verify the update
      await listPage.navigateToList('features');
      await listPage.expectRowContains('Updated Dashboard Analytics');
    });

    test('can change entity status', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      await listPage.navigateToList('features');
      await listPage.clickRowById('FEAT-001');

      if (!(await appPage.url()).includes('/form/')) {
        await appPage.click('a:has-text("Edit"), button:has-text("Edit")');
      }

      await formPage.selectField('status', 'done');
      await formPage.submit();

      // Verify status changed
      await listPage.navigateToList('features');
      await expect(appPage.locator('tr:has-text("FEAT-001")').locator('text=done')).toBeVisible();
    });
  });

  test.describe('Delete Entity', () => {
    test('can delete an entity from list', async ({ appPage }) => {
      const listPage = new ListPage(appPage);
      const formPage = new FormPage(appPage);

      // First create an entity to delete
      await listPage.navigateToList('features');
      await listPage.clickCreateButton();

      await formPage.fillField('title', 'Feature to Delete');
      await formPage.submit();

      // Navigate back to list
      await listPage.navigateToList('features');
      await listPage.expectRowContains('Feature to Delete');

      // Get the ID of the newly created feature
      const row = appPage.locator('.entity-row, tbody tr').filter({ hasText: 'Feature to Delete' });
      const rowText = await row.textContent();
      const idMatch = rowText?.match(/FEAT-\d+/);
      const featureId = idMatch ? idMatch[0] : 'Feature to Delete';

      // Delete it
      await listPage.deleteRowById(featureId);

      // Wait for update
      await appPage.waitForTimeout(500);

      // Verify it's gone
      await expect(appPage.locator('.entity-row, tbody tr').filter({ hasText: 'Feature to Delete' })).not.toBeVisible();
    });

    test('delete confirmation can be cancelled', async ({ appPage }) => {
      const listPage = new ListPage(appPage);

      await listPage.navigateToList('features');

      const initialCount = await listPage.getRowCount();

      // Set up dialog to be dismissed
      appPage.once('dialog', dialog => dialog.dismiss());

      // Try to delete
      const deleteBtn = appPage.locator('.entity-row, tbody tr').first().locator('.delete-btn, button[title="Delete"]');
      await deleteBtn.click();

      // Count should remain the same
      await expect(await listPage.getRowCount()).toBe(initialCount);
    });
  });
});
