import { type Locator, expect } from "@playwright/test";
import { BasePage } from "./base.page";

/** Page object for the relation history view
 *  (`/relation-history/:fromType/:from/:relType/:to`, RelationHistoryView.vue)
 *  and the per-relation History affordance on relation cards
 *  (RelationCards.vue, outgoing edges only). Selectors mirror the component's
 *  stable class names; see the template for `.timeline`, `.compare-select`,
 *  `.prop-diff`, etc. */
export class RelationHistoryPage extends BasePage {
  /** The History button rendered on an outgoing relation card whose target is
   *  `targetId`. Absent on incoming cards (the FROM entity owns the history). */
  historyButtonForCard(targetId: string): Locator {
    return this.page
      .locator(".relation-card", {
        has: this.page.locator(`.entity-id:has-text("${targetId}")`),
      })
      .locator(".history-btn");
  }

  /** Every History button in the currently-rendered relation widgets. */
  allHistoryButtons(): Locator {
    return this.page.locator(".relation-card .history-btn");
  }

  /** Open the history view by clicking the affordance on the outgoing card. */
  async openHistoryForCard(targetId: string) {
    await this.historyButtonForCard(targetId).click();
    await expect(this.page.locator(".history-view")).toBeVisible();
  }

  /** Wait until the timeline has at least `n` version rows (the sweep captures
   *  create/update asynchronously; the e2e server runs it on a short interval). */
  async waitForTimeline(n: number) {
    await expect(this.timelineItems()).toHaveCount(n, { timeout: 15_000 });
  }

  timelineItems(): Locator {
    return this.page.locator(".timeline .timeline-item");
  }

  async timelineCount(): Promise<number> {
    return this.timelineItems().count();
  }

  /** The op badges down the timeline, newest last (create/update/rename/delete). */
  async timelineOps(): Promise<string[]> {
    return this.page.locator(".timeline .timeline-badge").allTextContents();
  }

  async selectVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"] .timeline-select`)
      .click();
  }

  /** Choose the two compared versions in the diff pane and read back the
   *  rendered property changes. */
  async compare(baseVersion: number, targetVersion: number) {
    const selects = this.page.locator(".compare-select");
    await selects.nth(0).selectOption(String(baseVersion));
    await selects.nth(1).selectOption(String(targetVersion));
  }

  propDiffRows(): Locator {
    return this.page.locator(".prop-diff .prop-row");
  }

  async propDiffLabels(): Promise<string[]> {
    return this.page.locator(".prop-diff .prop-row .prop-label").allTextContents();
  }

  /** Click Restore on a specific version row (rows for op=delete have none). */
  async restoreVersion(version: number) {
    await this.page
      .locator(`.timeline-item[data-version="${version}"]`)
      .locator("button", { hasText: "Restore" })
      .click();
  }

  /** The "backend does not support version history" state (fsstore). */
  async expectUnsupported() {
    await expect(
      this.page.locator(".loading-state", { hasText: /does not support/i }),
    ).toBeVisible();
  }
}
