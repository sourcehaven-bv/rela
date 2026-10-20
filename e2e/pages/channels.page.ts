import { type Page, type Locator, expect } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * The channels panel on an entity page (TKT-91BK6F), as one user sees it.
 */
export class ChannelsPage extends BasePage {
  readonly panel: Locator;
  readonly channel: Locator;
  readonly composer: Locator;
  readonly messages: Locator;

  constructor(page: Page) {
    super(page);
    this.panel = page.locator('.channels-panel');
    this.channel = this.panel.locator('.rl-channel');
    this.composer = this.panel.getByRole('form', { name: 'New message' });
    this.messages = this.channel.locator('.rl-channel-message');
  }

  async openEntity(type: string, id: string) {
    await this.page.goto(`/entity/${type}/${id}`);
    await expect(this.page.locator('.detail-header')).toBeVisible({ timeout: 15_000 });
  }

  /** Waits for the open channel, titled `title`. */
  async expectChannel(title: string, timeout = 15_000) {
    await expect(this.channel.getByRole('heading', { name: title })).toBeVisible({ timeout });
  }

  async expectNoPanel(timeout = 15_000) {
    await expect(this.panel).toHaveCount(0, { timeout });
  }

  /** Writes `text` in the field labelled `field` and sends it. */
  async post(text: string, field = 'Message') {
    await this.composer.getByRole('textbox', { name: field }).fill(text);
    await this.composer.getByRole('button', { name: 'Send' }).click();
  }

  async expectComposerCleared(field = 'Message') {
    await expect(this.composer.getByRole('textbox', { name: field })).toHaveValue('');
  }

  async expectMessage(text: string, timeout = 15_000) {
    await expect(this.messages.filter({ hasText: text })).toHaveCount(1, { timeout });
  }

  async expectComposer(timeout = 15_000) {
    await expect(this.composer).toBeVisible({ timeout });
  }

  async expectNoComposer(timeout = 15_000) {
    await expect(this.composer).toHaveCount(0, { timeout });
  }

  /** Clicks a lifecycle button in the channel header, such as Close. */
  async runLifecycle(label: string) {
    await this.channel.locator('.rl-channel__lifecycle').getByRole('button', { name: label }).click();
  }

  async expectNoLifecycle(label: string) {
    await expect(
      this.channel.locator('.rl-channel__lifecycle').getByRole('button', { name: label }),
    ).toHaveCount(0);
  }

  async expectClosedNotice(timeout = 15_000) {
    await expect(this.channel).toContainText('This channel is closed.', { timeout });
  }
}
