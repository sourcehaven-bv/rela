import { test, expect } from './fixtures';

test.describe('Data Entry App', () => {
  test('loads and shows navigation', async ({ appPage }) => {
    // Check for navigation items from our test project (use role to be specific)
    await expect(appPage.getByRole('link', { name: 'Features' })).toBeVisible();
    await expect(appPage.getByRole('link', { name: 'Bugs' })).toBeVisible();
  });

  test('can navigate to Features list', async ({ appPage }) => {
    await appPage.click('text=Features');

    // Should show the features list
    await expect(appPage.locator('h1, h2').filter({ hasText: 'Features' })).toBeVisible();
  });

  test('can open create feature form', async ({ appPage }) => {
    // Navigate to features
    await appPage.click('text=Features');

    // Click create/new button
    const createButton = appPage.locator('a, button').filter({ hasText: /new|create|add/i }).first();
    await createButton.click();

    // Should show the feature form with title input
    await expect(appPage.locator('input[name="title"], input#title, input[id*="title"]')).toBeVisible();
  });

  test('can create a new feature', async ({ appPage }) => {
    // Navigate to features
    await appPage.click('text=Features');

    // Click create button
    const createButton = appPage.locator('a, button').filter({ hasText: /new|create|add/i }).first();
    await createButton.click();

    // Fill in the form
    await appPage.fill('input[name="title"], input#title, input[id*="title"]', 'Test Feature from E2E');

    // Submit the form
    await appPage.click('button[type="submit"], button:has-text("Save"), button:has-text("Create")');

    // Wait for navigation to complete
    await appPage.waitForURL(/.*/, { timeout: 10000 });

    // Verify the feature appears in the list (shown in table row)
    await appPage.click('text=Features');
    await expect(appPage.getByText('Test Feature from E2E').first()).toBeVisible();
  });

  test('can navigate to Bugs list', async ({ appPage }) => {
    await appPage.click('text=Bugs');

    // Should show the bugs list
    await expect(appPage.locator('h1, h2').filter({ hasText: 'Bugs' })).toBeVisible();
  });

  test('can create a new bug', async ({ appPage }) => {
    // Navigate to bugs
    await appPage.click('text=Bugs');

    // Click create button
    const createButton = appPage.locator('a, button').filter({ hasText: /new|create|add/i }).first();
    await createButton.click();

    // Fill in the form
    await appPage.fill('input[name="title"], input#title, input[id*="title"]', 'Test Bug from E2E');

    // Submit the form
    await appPage.click('button[type="submit"], button:has-text("Save"), button:has-text("Create")');

    // Wait for navigation to complete
    await appPage.waitForURL(/.*/, { timeout: 10000 });

    // Verify the bug appears in the list (shown in table row)
    await appPage.click('text=Bugs');
    await expect(appPage.getByText('Test Bug from E2E').first()).toBeVisible();
  });
});
