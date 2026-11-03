import { test, expect } from './fixtures';
import { EntityPage } from '../pages';

/**
 * Schema conversion: Review keeps the dialog open and shows the review step,
 * then Convert makes the new entity. The preview of a source without
 * relations once came back with null lists, the dialog failed to render, and
 * Review appeared to close it.
 */
test('Review shows the review step and Convert converts', async ({ appPage, api }) => {
  await api.createEntity('modules', { id: 'MOD-CONV', properties: { name: 'Billing' } });
  const page = new EntityPage(appPage);
  await page.navigateToEntity('module', 'MOD-CONV');

  await page.openConversion('Convert to feature');
  await page.pressConversionAction('Review');
  await expect(page.conversionDialog).toContainText('Review done');

  await page.pressConversionAction('Convert');
  await expect(page.conversionDialog).toBeHidden();
  await expect(appPage).toHaveURL(/\/entity\/feature\/FEAT-/);
  await expect(page.heading).toContainText('Billing');
});
