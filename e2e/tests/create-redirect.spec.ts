import { test, expect } from './fixtures';

test.describe('Create entity redirect', () => {
  test('detail page loads after create without error', async ({ appPage, serverUrl }) => {
    await appPage.goto(`${serverUrl}/form/feature`);
    await expect(appPage.locator('#field-title')).toBeVisible({ timeout: 10000 });

    await appPage.locator('#field-title').fill('Redirect Test Feature');
    await appPage.locator('#field-status').selectOption('draft');
    await appPage.locator('#field-priority').selectOption('high');

    const [response] = await Promise.all([
      appPage.waitForResponse(resp => resp.url().includes('/api/v1/features') && resp.status() === 201),
      appPage.locator('button[type="submit"]').click(),
    ]);
    expect(response.ok()).toBeTruthy();

    await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-\d+/, { timeout: 10000 });
    await expect(appPage.locator('h1').filter({ hasText: 'Redirect Test Feature' })).toBeVisible({ timeout: 10000 });
    await expect(appPage.getByText('Entity not found')).not.toBeVisible();
    await expect(appPage.getByText('Failed to load')).not.toBeVisible();
  });

  test('detail page loads after rapid create (stress)', async ({ appPage, serverUrl }) => {
    for (let i = 1; i <= 3; i++) {
      await appPage.goto(`${serverUrl}/form/feature`);
      await expect(appPage.locator('#field-title')).toBeVisible({ timeout: 10000 });

      await appPage.locator('#field-title').fill(`Stress Test Feature ${i}`);
      await appPage.locator('#field-status').selectOption('draft');

      const [response] = await Promise.all([
        appPage.waitForResponse(resp => resp.url().includes('/api/v1/features') && resp.status() === 201),
        appPage.locator('button[type="submit"]').click(),
      ]);
      expect(response.ok()).toBeTruthy();

      await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-\d+/, { timeout: 10000 });
      await expect(appPage.locator('h1').filter({ hasText: `Stress Test Feature ${i}` })).toBeVisible({ timeout: 10000 });
      await expect(appPage.getByText('Entity not found')).not.toBeVisible();
      await expect(appPage.getByText('Failed to load')).not.toBeVisible();
    }
  });
});
