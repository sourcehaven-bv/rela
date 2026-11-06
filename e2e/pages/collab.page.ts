import { type Page, type Locator, expect } from '@playwright/test';
import { EntityPage } from './entity.page';

/**
 * One person's view of an entity body that several people edit at once
 * (TKT-CJL604). It counts the body saves this page sends, so a spec can
 * check that only the elected saver writes.
 */
// Home and End do not move the caret in a contenteditable on macOS.
const MAC = process.platform === 'darwin';
const LINE_START = MAC ? 'Meta+ArrowLeft' : 'Home';
const LINE_END = MAC ? 'Meta+ArrowRight' : 'End';

export class CollabBodyPage extends EntityPage {
  private saves = 0;

  constructor(page: Page, entityId: string) {
    super(page);
    page.on('request', (req) => {
      if (req.method() === 'PATCH' && req.url().includes(`/${entityId}`)) this.saves++;
    });
  }

  /** The labelled cursor of a peer named `user`. */
  peerCursor(user: string): Locator {
    return this.bodyEditor.locator('.ProseMirror-yjs-cursor', { hasText: user });
  }

  /** The body saves this page has sent. */
  get saveCount(): number {
    return this.saves;
  }

  /** Opens the body editor and waits until the room has synced: the editor
   *  is read-only until then. */
  async openEditor(type: string, id: string) {
    // A fresh page is at about:blank, so go through the context's baseURL.
    await this.page.goto(`/entity/${type}/${id}`);
    await expect(this.heading).toBeVisible({ timeout: 15_000 });
    await this.contentBody.hover();
    await this.bodyEditButton.click();
    await expect(this.bodyEditor).toHaveAttribute('contenteditable', 'true', { timeout: 15_000 });
  }

  /** Waits past the 5.8s after which a client that is not the saver saves
   *  text the room has not saved (useBodyCollab's FALLBACK_MS). */
  async waitPastSaveFallback() {
    await this.page.waitForTimeout(6_500);
  }

  /** Puts the caret at the end of the paragraph holding `text`. */
  async caretAfter(text: string) {
    await this.bodyEditor.getByText(text).first().click();
    await this.page.keyboard.press(LINE_END);
  }

  /** Puts the caret at the start of the paragraph holding `text`. */
  async caretAtStartOf(text: string) {
    await this.bodyEditor.getByText(text).first().click();
    await this.page.keyboard.press(LINE_START);
  }

  async type(text: string) {
    await this.page.keyboard.type(text, { delay: 20 });
  }

  /** The editor's text, one line per block, without peer cursor labels. */
  async bodyText(): Promise<string> {
    return this.bodyEditor.evaluate((el) => {
      const copy = el.cloneNode(true) as HTMLElement;
      copy.querySelectorAll('.ProseMirror-yjs-cursor').forEach((c) => c.remove());
      return Array.from(copy.children, (b) => (b.textContent ?? '').trim()).join('\n');
    });
  }
}
