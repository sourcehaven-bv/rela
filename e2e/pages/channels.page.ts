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

  /** Types into the composer without sending. */
  async draft(text: string, field = 'Message') {
    await this.composer.getByRole('textbox', { name: field }).fill(text);
  }

  /** The typing line under the log reads `text`. */
  async expectTyping(text: string, timeout = 15_000) {
    await expect(this.channel.locator('.rl-channel__typing')).toHaveText(text, { timeout });
  }

  /** Nobody else is typing. */
  async expectNoTyping(timeout = 15_000) {
    await expect(this.channel.locator('.rl-channel__typing')).toHaveText('', { timeout });
  }

  async expectComposerCleared(field = 'Message') {
    await expect(this.composer.getByRole('textbox', { name: field })).toHaveValue('');
  }

  async expectMessage(text: string, timeout = 15_000) {
    await expect(this.messages.filter({ hasText: text })).toHaveCount(1, { timeout });
  }

  async expectNoMessage(text: string) {
    await expect(this.messages.filter({ hasText: text })).toHaveCount(0);
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

  /** The session stepper of the open channel. */
  get session(): Locator {
    return this.channel.locator('.rl-channel-session');
  }

  /** The stepper marks `label` as the current state. */
  async expectSessionAt(label: string, timeout = 15_000) {
    await expect(this.session.locator('[aria-current="step"]')).toContainText(label, { timeout });
  }

  /** The session's live region holds the activity line with `text`, or is empty of it. */
  async expectActivityLine(text: string, shown: boolean, timeout = 15_000) {
    const line = this.session.getByRole('status').filter({ hasText: text });
    await expect(line).toHaveCount(shown ? 1 : 0, { timeout });
  }

  /** Selects the message type `label` in the composer's type picker. */
  async selectMessageType(label: string) {
    await this.composer.getByRole('radio', { name: new RegExp(`^${label}\\b`) }).click();
  }

  /** The composer offers the message type `label`, or does not. */
  async expectMessageType(label: string, offered: boolean) {
    await expect(this.composer.getByRole('radio', { name: new RegExp(`^${label}\\b`) })).toHaveCount(offered ? 1 : 0);
  }

  /** The composer marks the message type `label` as one that moves the session, or does not. */
  async expectTransitionMark(label: string, marked: boolean, timeout = 15_000) {
    const name = new RegExp(`^${label}\\b.*moves the session`);
    await expect(this.composer.getByRole('radio', { name })).toHaveCount(marked ? 1 : 0, { timeout });
  }

  /** The composer's note on where sending moves the session; null: no note. */
  async expectSessionHint(text: string | null) {
    if (text === null) await expect(this.composer).not.toContainText('moves the session to');
    else await expect(this.composer).toContainText(text);
  }

  /** The message holding `text` offers the control `label`, or does not. */
  async expectMessageControl(text: string, label: string, offered: boolean) {
    const row = this.messages.filter({ hasText: text });
    await expect(row.getByRole('button', { name: label, exact: true })).toHaveCount(offered ? 1 : 0);
  }

  // --- Session decisions (TKT-T9G1AE) ---------------------------------------

  /** The decision button `label` in `scope`; its name also holds a hidden hint. */
  private decisionIn(scope: Locator, label: string): Locator {
    return scope
      .getByRole('group', { name: 'Decisions' })
      .getByRole('button', { name: new RegExp(`^${label}\\b`) });
  }

  /** The session header shows the decision `label`, or does not. */
  async expectDecision(label: string, shown: boolean, timeout = 15_000) {
    await expect(this.decisionIn(this.session, label)).toHaveCount(shown ? 1 : 0, { timeout });
  }

  /** Clicks the decision `label` in the session header. */
  async decide(label: string) {
    await this.decisionIn(this.session, label).click();
  }

  /** The message holding `text` shows the decision `label`, or does not. */
  async expectDecisionOnMessage(text: string, label: string, shown: boolean, timeout = 15_000) {
    const row = this.messages.filter({ hasText: text });
    await expect(this.decisionIn(row, label)).toHaveCount(shown ? 1 : 0, { timeout });
  }

  /** Clicks the decision `label` on the message holding `text`. */
  async decideOnMessage(text: string, label: string) {
    await this.decisionIn(this.messages.filter({ hasText: text }), label).click();
  }

  /** Accepts the in-app confirm dialog with the button `label`. */
  async confirmDialog(label?: string) {
    const dialog = this.page.locator('[role=dialog], [role=alertdialog]').last();
    await expect(dialog).toBeVisible({ timeout: 15_000 });
    const buttons = dialog.getByRole('button');
    await (label ? buttons.filter({ hasText: new RegExp(`^${label}$`) }) : buttons).last().click();
  }

  // --- Choices (TKT-03FQPD) -------------------------------------------------

  /**
   * Posts a message over the API as this page's user, for a property the
   * composer cannot enter, such as a list. `plural` and `id` name the entity.
   */
  async postOverApi(plural: string, id: string, channelType: string, type: string, props: Record<string, unknown>) {
    const origin = new URL(this.page.url()).origin;
    const resp = await this.page.request.post(`${origin}/api/v1/${plural}/${id}/_channels/${channelType}/messages`, {
      data: { type, props },
      headers: { Origin: origin },
    });
    expect(resp.status(), `post ${type}: ${await resp.text()}`).toBe(201);
  }

  /** The choice button `label` on the message holding `text`; its name also holds a hidden hint. */
  private choiceOn(text: string, label: string): Locator {
    return this.messages
      .filter({ hasText: text })
      .getByRole('group', { name: 'Choices' })
      .getByRole('button', { name: new RegExp(`^${label}(?: \\(|$)`) });
  }

  /** The message holding `text` shows the choice `label`, or does not. */
  async expectChoice(text: string, label: string, shown: boolean, timeout = 15_000) {
    await expect(this.choiceOn(text, label)).toHaveCount(shown ? 1 : 0, { timeout });
  }

  /** Clicks the choice `label` on the message holding `text`. */
  async choose(text: string, label: string) {
    await this.choiceOn(text, label).click();
  }

  /** The message holding `text` lists `items` as a bulleted list. */
  async expectListedOptions(text: string, items: string[], timeout = 15_000) {
    const list = this.messages.filter({ hasText: text }).locator('.rl-channel-message__list li');
    await expect(list).toHaveText(items, { timeout });
  }

  /** A message that replies to the one holding `quoted` shows `text`. */
  async expectReply(quoted: string, text: string, timeout = 15_000) {
    const row = this.messages.filter({ has: this.page.locator('.rl-channel-message__quote', { hasText: quoted }) });
    await expect(row.filter({ hasText: text })).toHaveCount(1, { timeout });
  }

  // --- Inbox (TKT-9YRRM5) ---------------------------------------------------

  /** The pinned Inbox entry in the sidebar. */
  get inboxItem(): Locator {
    return this.page.locator('.rl-sidebar a[href="/inbox"]');
  }

  /** Opens the app at its start page and waits for the Inbox entry. */
  async openHome() {
    await this.page.goto('/');
    await expect(this.inboxItem).toBeVisible({ timeout: 15_000 });
  }

  /** The Inbox badge shows `count`. */
  async expectInboxBadge(count: number, timeout = 15_000) {
    await expect(this.inboxItem.locator('.rl-nav-status__count')).toHaveText(String(count), {
      timeout,
    });
  }

  /** The Inbox entry shows no badge at all. */
  async expectNoInboxBadge(timeout = 15_000) {
    await expect(this.inboxItem.locator('.rl-nav-status')).toHaveCount(0, { timeout });
  }

  /** Opens the inbox page from the sidebar. */
  async openInbox() {
    await this.inboxItem.click();
    await expect(this.page).toHaveURL(/\/inbox$/);
    await expect(this.page.locator('.rl-inbox')).toBeVisible({ timeout: 15_000 });
  }

  /** Clicks the row of the inbox section `section` that reads `text`. */
  async openInboxRow(section: 'waiting' | 'mentions' | 'unread', text: string) {
    await this.page
      .locator(`.rl-inbox li[data-section="${section}"]`)
      .filter({ hasText: text })
      .getByRole('link')
      .click();
  }

  /**
   * The page is the entity page with the deep link to `channelKey`. A
   * message link arrives with `&seq=`, which the page drops once it has
   * revealed the message; the router then writes the key unencoded, so
   * both spellings of `/` are accepted.
   */
  async expectChannelDeepLink(type: string, id: string, channelKey: string) {
    const key = encodeURIComponent(channelKey).replace(/%2F/g, '(?:%2F|/)');
    await expect(this.page).toHaveURL(new RegExp(`/entity/${type}/${id}\\?channel=${key}$`));
  }
}
