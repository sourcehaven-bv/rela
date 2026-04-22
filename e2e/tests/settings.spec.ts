import { test, expect } from './fixtures';
import { SettingsPage } from '../pages/settings.page';

test.describe('Settings', () => {
  test.describe('Display', () => {
    test('settings page loads', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();
      await settingsPage.expectHeading('Settings');
    });

    test('shows property defaults section', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.propertyDefaultsCard).toBeVisible();
    });

    test('shows relation defaults section', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.relationDefaultsCard).toBeVisible();
    });

    test('shows overrides section', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.overridesCard).toBeVisible();
    });

    test('shows app info', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.appInfoCard).toBeVisible();
      await settingsPage.expectAppInfo('App Name', 'E2E Test App');
    });
  });

  test.describe('Property Defaults', () => {
    test('can add a property default', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();
      await settingsPage.addFirstAvailablePropertyDefault();

      expect(await settingsPage.getPropertyDefaultRowCount()).toBeGreaterThan(0);
    });

    test('can set property default value', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();
      await settingsPage.addFirstAvailablePropertyDefault();
      // Use any non-empty value; the row's input picks whichever control renders
      // for the property type (text field or enum select).
      await settingsPage.setLastPropertyDefaultValue('default value');
    });

    test('can remove property default', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();
      await settingsPage.addFirstAvailablePropertyDefault();

      const initialCount = await settingsPage.getPropertyDefaultRowCount();
      await settingsPage.removeFirstPropertyDefault();

      expect(await settingsPage.getPropertyDefaultRowCount()).toBe(initialCount - 1);
    });
  });

  test.describe('Override Groups', () => {
    test('can add override group', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      const initialCount = await settingsPage.getOverrideGroupCount();

      await settingsPage.addOverrideGroup();

      const newCount = await settingsPage.getOverrideGroupCount();
      expect(newCount).toBe(initialCount + 1);
    });

    test('can remove override group', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      // Add a group first
      await settingsPage.addOverrideGroup();
      const countAfterAdd = await settingsPage.getOverrideGroupCount();

      // Remove it
      await settingsPage.removeOverrideGroup(countAfterAdd - 1);

      const countAfterRemove = await settingsPage.getOverrideGroupCount();
      expect(countAfterRemove).toBe(countAfterAdd - 1);
    });
  });

  test.describe('Save and Reset', () => {
    test('save button is visible', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.saveButton).toBeVisible();
    });

    test('reset button is visible', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await expect(settingsPage.resetButton).toBeVisible();
    });

    test('can save settings', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      // Make a change
      await settingsPage.addOverrideGroup();

      // Save
      await settingsPage.save();

      // Should show success message
      await settingsPage.expectSaveSuccess();
    });

    test('reset reverts changes', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      const initialCount = await settingsPage.getOverrideGroupCount();

      // Add a group
      await settingsPage.addOverrideGroup();

      // Reset
      await settingsPage.reset();

      // Should be back to initial count
      const countAfterReset = await settingsPage.getOverrideGroupCount();
      expect(countAfterReset).toBe(initialCount);
    });
  });

  test.describe('App Info', () => {
    test('shows entity type count', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await settingsPage.expectAppInfo('Entity Types', '3'); // feature, bug, task
    });

    test('shows relation type count', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await settingsPage.expectAppInfo('Relation Types', '3'); // blocks, implements, fixes
    });

    test('shows forms count', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await settingsPage.expectAppInfo('Forms', '3'); // feature, bug, task
    });

    test('shows lists count', async ({ appPage }) => {
      const settingsPage = new SettingsPage(appPage);

      await settingsPage.navigateToSettings();

      await settingsPage.expectAppInfo('Lists', '3'); // features, bugs, tasks
    });
  });
});
