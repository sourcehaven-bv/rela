import fs from 'node:fs';
import path from 'node:path';
import { test as base, expect } from './fixtures';
import { AppShellPage, DashboardPage, SettingsPage } from '../pages';

/**
 * Settings for a user without settings:edit, and the `builtins.settings` /
 * `builtins.about` toggles. Both are UX choices: the configuration is not a
 * secret, and every settings save checks the permission itself.
 */

// The e2e user may read and write everything but does not hold settings:edit.
const NO_SETTINGS_ACL = `roles:
  customer:
    create: ["*"]
    read: ["*"]
    update: ["*"]
assignments:
  e2e@example.com: customer
`;

const BUILTINS_YAML = `
builtins:
  settings: { permission: "settings:edit" }
  about: { hidden: true }
`;

const ABOUT_YAML = `
  about:
    version: "2026.11"
    links:
      - { label: Help, url: "https://help.example.com/" }
      - { label: Terms, url: /terms }
`;

const testNoSettings = base.extend({
  testProject: async ({ testProject }, use) => {
    fs.writeFileSync(path.join(testProject, 'acl.yaml'), NO_SETTINGS_ACL);
    await use(testProject);
  },
});

const testBuiltins = testNoSettings.extend({
  testProject: async ({ testProject }, use) => {
    fs.appendFileSync(path.join(testProject, 'data-entry.yaml'), BUILTINS_YAML);
    await use(testProject);
  },
});

// Inserts `about:` under the existing top-level `app:` block.
const testAbout = base.extend({
  testProject: async ({ testProject }, use) => {
    const file = path.join(testProject, 'data-entry.yaml');
    const yaml = fs.readFileSync(file, 'utf8');
    fs.writeFileSync(file, yaml.replace(/^app:\n/m, `app:\n${ABOUT_YAML.slice(1)}`));
    await use(testProject);
  },
});

base('a settings editor gets the shared settings', async ({ appPage }) => {
  const settings = new SettingsPage(appPage);
  await settings.navigateToSettings();
  await settings.expectEditorControls(true);
});

testNoSettings('without settings:edit the page shows only the own preferences', async ({ appPage }) => {
  const settings = new SettingsPage(appPage);
  await settings.navigateToSettings();
  await settings.expectEditorControls(false);
});

testBuiltins('builtins hide Settings and About from the footer', async ({ appPage }) => {
  const dashboard = new DashboardPage(appPage);
  await dashboard.navigate();
  const shell = new AppShellPage(appPage);
  await shell.expectSettingsAndAbout(false, false);
});

testAbout('About shows the configured version and links', async ({ appPage }) => {
  const dashboard = new DashboardPage(appPage);
  await dashboard.navigate();
  const shell = new AppShellPage(appPage);
  const about = await shell.readAbout();
  expect(about.version).toBe('Version 2026.11');
  expect(about.links).toEqual(['Help', 'Terms']);
});
